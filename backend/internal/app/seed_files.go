package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RestoreSeedMaterialFiles recreates only the two known bundled seed files when
// they are missing. Existing files are never overwritten.
func RestoreSeedMaterialFiles(ctx context.Context, store *Store, uploadDir string) error {
	if err := os.MkdirAll(uploadDir, 0750); err != nil {
		return fmt.Errorf("create upload directory for bundled materials: %w", err)
	}
	rows, err := store.DB.QueryContext(ctx, `SELECT m.id, m.stored_filename, k.content FROM materials m JOIN knowledge_entries k ON k.material_id=m.id AND k.class_id=m.class_id WHERE (m.id=1 AND m.class_id=1 AND m.stored_filename='seed-a.txt') OR (m.id=2 AND m.class_id=2 AND m.stored_filename='seed-b.txt')`)
	if err != nil { return err }
	type seedFile struct { id int64; name, content string }
	seeds := []seedFile{}
	for rows.Next() {
		var item seedFile
		if err := rows.Scan(&item.id, &item.name, &item.content); err != nil { rows.Close(); return err }
		seeds = append(seeds, item)
	}
	if err := rows.Err(); err != nil { rows.Close(); return err }
	if err := rows.Close(); err != nil { return err }
	for _, seed := range seeds {
		path := filepath.Join(uploadDir, seed.name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if errors.Is(err, os.ErrExist) { continue }
		if err != nil { return fmt.Errorf("restore bundled material %d: %w", seed.id, err) }
		_, writeErr := file.WriteString(seed.content)
		if writeErr == nil { writeErr = file.Sync() }
		closeErr := file.Close()
		if writeErr != nil { _ = os.Remove(path); return fmt.Errorf("restore bundled material %d: %w", seed.id, writeErr) }
		if closeErr != nil { _ = os.Remove(path); return fmt.Errorf("restore bundled material %d: %w", seed.id, closeErr) }
	}
	return nil
}
