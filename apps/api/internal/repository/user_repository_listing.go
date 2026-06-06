package repository

import "context"

// ListActiveUserIdentifiers returns the ids of all active users, for background automation to iterate.
func (repository *PostgresUserRepository) ListActiveUserIdentifiers(loadContext context.Context) ([]int64, error) {
	rows, queryError := repository.Database.QueryContext(loadContext, "SELECT id FROM users WHERE is_active = true ORDER BY id")
	if queryError != nil {
		return nil, queryError
	}
	defer rows.Close()

	userIdentifiers := make([]int64, 0)
	for rows.Next() {
		var userIdentifier int64
		if scanError := rows.Scan(&userIdentifier); scanError != nil {
			return nil, scanError
		}
		userIdentifiers = append(userIdentifiers, userIdentifier)
	}
	return userIdentifiers, rows.Err()
}
