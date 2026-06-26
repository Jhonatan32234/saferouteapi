package repository

import (
	"database/sql"
	"fmt"

	"saferoute/entities"
)

// UsuarioRepository define las operaciones de acceso a datos para la tabla `usuarios`.
// Todas las consultas usan prepared statements con placeholders ($1, $2...).
type UsuarioRepository struct {
	db            *sql.DB
	encryptionKey []byte
}

// NewUsuarioRepository crea una nueva instancia del repositorio.
func NewUsuarioRepository(db *sql.DB, encryptionKey []byte) *UsuarioRepository {
	return &UsuarioRepository{db: db, encryptionKey: encryptionKey}
}

// FindByEmail busca un usuario por email. Aplica AfterLoad para descifrar el teléfono.
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
	// Decorator/Hook: descifrar campo sensible al cargar desde BD
	if err := u.AfterLoad(r.encryptionKey); err != nil {
		return nil, fmt.Errorf("AfterLoad error: %w", err)
	}
	return u, nil
}

// FindByID busca un usuario por su UUID. Aplica AfterLoad para descifrar el teléfono.
func (r *UsuarioRepository) FindByID(id string) (*entities.UsuarioEntity, error) {
	u := &entities.UsuarioEntity{}
	var ultimoAcceso sql.NullTime
	err := r.db.QueryRow(
		`SELECT id, email, password_hash, nombre, tipo, COALESCE(telefono, ''),
		        created_at, updated_at, ultimo_acceso
		 FROM usuarios WHERE id = $1`,
		id,
	).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Nombre,
		&u.Tipo, &u.Telefono, &u.CreatedAt, &u.UpdatedAt, &ultimoAcceso,
	)
	if err != nil {
		return nil, err
	}
	if ultimoAcceso.Valid {
		t := ultimoAcceso.Time
		u.UltimoAcceso = &t
	}
	// Decorator/Hook: descifrar campo sensible al cargar desde BD
	if err := u.AfterLoad(r.encryptionKey); err != nil {
		return nil, fmt.Errorf("AfterLoad error: %w", err)
	}
	return u, nil
}

// Create inserta un nuevo usuario. Aplica BeforeSave para cifrar el teléfono.
// Recibe la entidad con datos en claro y devuelve el ID generado por PostgreSQL.
func (r *UsuarioRepository) Create(u *entities.UsuarioEntity) (string, error) {
	// Decorator/Hook: cifrar campo sensible antes de persistir
	if err := u.BeforeSave(r.encryptionKey); err != nil {
		return "", fmt.Errorf("BeforeSave error: %w", err)
	}

	var id string
	err := r.db.QueryRow(
		`INSERT INTO usuarios (email, password_hash, nombre, tipo, telefono)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		 RETURNING id`,
		u.Email, u.PasswordHash, u.Nombre, u.Tipo, u.Telefono,
	).Scan(&id)
	return id, err
}

// Update actualiza los campos de perfil de un usuario. Aplica BeforeSave para cifrar el teléfono.
func (r *UsuarioRepository) Update(u *entities.UsuarioEntity) error {
	// Decorator/Hook: cifrar campo sensible antes de persistir
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
	if u.Telefono != "" {
		argCount++
		query += fmt.Sprintf(", telefono = $%d", argCount)
		args = append(args, u.Telefono)
	}
	if u.Email != "" {
		argCount++
		query += fmt.Sprintf(", email = $%d", argCount)
		args = append(args, u.Email)
	}

	argCount++
	query += fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, u.ID)

	_, err := r.db.Exec(query, args...)
	return err
}

// UpdateLastAccess actualiza solo el campo ultimo_acceso del usuario.
func (r *UsuarioRepository) UpdateLastAccess(userID string) error {
	_, err := r.db.Exec(
		"UPDATE usuarios SET ultimo_acceso = NOW() WHERE id = $1",
		userID,
	)
	return err
}
