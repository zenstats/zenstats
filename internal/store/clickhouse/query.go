package clickhouse

// // Query executes a SELECT query and returns the rows.
// func Query(query string, args ...any) (*clickhouse.Rows, error) {
// 	ctx := context.Background()
// 	rows, err := GetConnection().Query(ctx, query, args...)
// 	if err != nil {
// 		log.Printf("Query error: %v", err)
// 		return nil, err
// 	}
// 	return rows, nil
// }

// // Exec executes a non-SELECT query (INSERT, UPDATE, DELETE, etc.).
// func Exec(query string, args ...any) error {
// 	ctx := context.Background()
// 	_, err := GetConnection().Exec(ctx, query, args...)
// 	if err != nil {
// 		log.Printf("Exec error: %v", err)
// 		return err
// 	}
// 	return nil
// }
