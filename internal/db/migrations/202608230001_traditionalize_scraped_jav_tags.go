package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"javboss/internal/util"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddNamedMigrationContext(
		"202608230001_traditionalize_scraped_jav_tags.go",
		traditionalizeScrapedJavTags,
		irreversibleMigration,
	)
}

type scrapedJavTagMigrationRow struct {
	id         int64
	name       string
	categoryID sql.NullInt64
}

type scrapedJavTagMigrationGroup struct {
	canonical string
	rows      []scrapedJavTagMigrationRow
}

func traditionalizeScrapedJavTags(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, COALESCE(name, ''), category_id
		 FROM jav_tag
		 WHERE COALESCE(is_user, 0) = 0
		 ORDER BY id`,
	)
	if err != nil {
		return fmt.Errorf("load scraped JAV tags: %w", err)
	}

	groupsByName := make(map[string]*scrapedJavTagMigrationGroup)
	groups := make([]*scrapedJavTagMigrationGroup, 0)
	for rows.Next() {
		var row scrapedJavTagMigrationRow
		if err := rows.Scan(&row.id, &row.name, &row.categoryID); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan scraped JAV tag: %w", err)
		}
		row.name = strings.TrimSpace(row.name)
		canonical, err := util.TraditionalizeChineseName(row.name)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("traditionalize scraped JAV tag %d (%q): %w", row.id, row.name, err)
		}
		group := groupsByName[canonical]
		if group == nil {
			group = &scrapedJavTagMigrationGroup{canonical: canonical}
			groupsByName[canonical] = group
			groups = append(groups, group)
		}
		group.rows = append(group.rows, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate scraped JAV tags: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close scraped JAV tags: %w", err)
	}

	for _, group := range groups {
		if err := mergeScrapedJavTagGroup(ctx, tx, group); err != nil {
			return err
		}
	}
	return nil
}

func mergeScrapedJavTagGroup(ctx context.Context, tx *sql.Tx, group *scrapedJavTagMigrationGroup) error {
	keeperIndex := 0
	for i, row := range group.rows {
		if row.name == group.canonical {
			keeperIndex = i
			break
		}
	}
	keeper := group.rows[keeperIndex]
	categoryID := keeper.categoryID
	if !categoryID.Valid {
		for _, row := range group.rows {
			if row.categoryID.Valid {
				categoryID = row.categoryID
				break
			}
		}
	}

	for i, duplicate := range group.rows {
		if i == keeperIndex {
			continue
		}
		if err := execDB(
			ctx,
			tx,
			`INSERT OR IGNORE INTO jav_tag_map (jav_id, jav_tag_id, provider, created_at)
			 SELECT jav_id, ?, provider, created_at
			 FROM jav_tag_map
			 WHERE jav_tag_id = ?`,
			keeper.id,
			duplicate.id,
		); err != nil {
			return fmt.Errorf("move JAV tag maps from %d to %d: %w", duplicate.id, keeper.id, err)
		}
		if err := execDB(ctx, tx, `DELETE FROM jav_tag_map WHERE jav_tag_id = ?`, duplicate.id); err != nil {
			return fmt.Errorf("delete JAV tag maps for %d: %w", duplicate.id, err)
		}
		if err := execDB(ctx, tx, `DELETE FROM jav_tag WHERE id = ?`, duplicate.id); err != nil {
			return fmt.Errorf("delete duplicate JAV tag %d: %w", duplicate.id, err)
		}
	}

	var categoryValue any
	if categoryID.Valid {
		categoryValue = categoryID.Int64
	}
	if err := execDB(
		ctx,
		tx,
		`UPDATE jav_tag SET name = ?, category_id = ? WHERE id = ?`,
		group.canonical,
		categoryValue,
		keeper.id,
	); err != nil {
		return fmt.Errorf("update canonical JAV tag %d: %w", keeper.id, err)
	}
	return nil
}
