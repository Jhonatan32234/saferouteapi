package repository

import (
	"database/sql"
	"fmt"
	"log"

	"saferoute/entities"
)


type UsuarioRepository struct {
	db            *sql.DB
	encryptionKey []byte
}

func NewUsuarioRepository(db *sql.DB, encryptionKey []byte) *UsuarioRepository {
	return &UsuarioRepository{db: db, encryptionKey: encryptionKey}
}

func (r *UsuarioRepository) FindByEmail(email string) (*entities.UsuarioEntity, error) {
	u := &entities.UsuarioEntity{}
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, nombre, tipo, COALESCE(telefono, ''), created_at, updated_at
		 FROM usuarios WHERE email = $1`,
		email,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nombre,
		&u.Tipo, &u.Telefono, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := u.AfterLoad(r.encryptionKey); err != nil {
		return nil, fmt.Errorf("AfterLoad error: %w", err)
	}
	return u, nil
}

func (r *UsuarioRepository) FindByID(id string) (*entities.UsuarioPerfilConEstadisticas, error) {
    u := &entities.UsuarioPerfilConEstadisticas{}
    var ultimoAcceso sql.NullTime

    err := r.db.QueryRow(
        `SELECT u.id, u.email, u.password_hash, u.nombre, u.tipo, COALESCE(u.telefono, ''),
                u.created_at, u.updated_at, u.ultimo_acceso,
                COALESCE(COUNT(r.id), 0) AS reportes_creados,
                COALESCE(SUM(CASE WHEN r.confirmaciones > 0 THEN 1 ELSE 0 END), 0) AS reportes_confirmados
         FROM usuarios u
         LEFT JOIN reportes r ON r.user_id = u.id
         WHERE u.id = $1
         GROUP BY u.id, u.email, u.password_hash, u.nombre, u.tipo, u.telefono,
                  u.created_at, u.updated_at, u.ultimo_acceso`,
        id,
    ).Scan(
        &u.ID, &u.Email, &u.PasswordHash, &u.Nombre,
        &u.Tipo, &u.Telefono, &u.CreatedAt, &u.UpdatedAt, &ultimoAcceso,
        &u.ReportesCreados, &u.ReportesConfirmados,
    )
    if err != nil {
        return nil, err
    }

    if ultimoAcceso.Valid {
        t := ultimoAcceso.Time
        u.UltimoAcceso = &t
    }

    if err := u.AfterLoad(r.encryptionKey); err != nil {
        return nil, fmt.Errorf("AfterLoad error: %w", err)
    }

    return u, nil
}


func (r *UsuarioRepository) Create(u *entities.UsuarioEntity) (string, error) {
    // LOG para debug
    log.Printf("[REPO] Creando usuario - Email: %s, Teléfono antes de BeforeSave: '%s'", 
        u.Email, u.Telefono)

    // Cifrar teléfono antes de guardar
    if err := u.BeforeSave(r.encryptionKey); err != nil {
        return "", fmt.Errorf("BeforeSave error: %w", err)
    }

    log.Printf("[REPO] Teléfono después de BeforeSave: '%s'", u.Telefono)

    var id string
    err := r.db.QueryRow(
        `INSERT INTO usuarios (email, password_hash, nombre, tipo, telefono)
         VALUES ($1, $2, $3, $4, $5)
         RETURNING id`,
        u.Email, u.PasswordHash, u.Nombre, u.Tipo, u.Telefono,
    ).Scan(&id)
    
    if err != nil {
        log.Printf("[REPO] Error INSERT: %v", err)
        return "", err
    }

    log.Printf("[REPO] Usuario creado - ID: %s", id)
    return id, nil
}
func (r *UsuarioRepository) Update(u *entities.UsuarioEntity) error {
    if err := u.BeforeSave(r.encryptionKey); err != nil {
        return fmt.Errorf("BeforeSave error: %w", err)
    }

    query := "UPDATE usuarios SET updated_at = NOW()"
    args := []interface{}{}
    argCount := 0

    if u.Nombre != "" {
        argCount++
        query += fmt.Sprintf(", nombre = $%d", argCount)
        args = append(args, u.Nombre)
    }
    
    argCount++
    query += fmt.Sprintf(", telefono = NULLIF($%d, '')", argCount)
    args = append(args, u.Telefono)
    
    if u.Email != "" {
        argCount++
        query += fmt.Sprintf(", email = $%d", argCount)
        args = append(args, u.Email)
    }

    argCount++
    query += fmt.Sprintf(" WHERE id = $%d", argCount)
    args = append(args, u.ID)

    log.Printf("[REPO] Update query: %s", query)
    log.Printf("[REPO] Update args: %v", args)

    result, err := r.db.Exec(query, args...)
    if err != nil {
        return err
    }

    rows, _ := result.RowsAffected()
    log.Printf("[REPO] Filas actualizadas: %d", rows)
    
    return nil
}

func (r *UsuarioRepository) UpdateLastAccess(userID string) error {
	_, err := r.db.Exec(
		"UPDATE usuarios SET ultimo_acceso = NOW() WHERE id = $1",
		userID,
	)
	return err
}
