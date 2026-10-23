package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const noResultsMessage = "资料中未找到相关内容"

type RetrievalService struct {
	store   *Store
	models  *ModelClient
	qdrant  *QdrantClient
	config  Config
	indexMu sync.Mutex
}

type SearchHit struct {
	MaterialID       int64   `json:"material_id"`
	MaterialTitle    string  `json:"material_title"`
	KnowledgeEntryID int64   `json:"knowledge_entry_id"`
	ChunkID          int64   `json:"chunk_id"`
	ChunkIndex       int     `json:"chunk_index"`
	StartOffset      int     `json:"start_offset"`
	EndOffset        int     `json:"end_offset"`
	Excerpt          string  `json:"excerpt"`
	MaterialURL      string  `json:"material_url"`
	Rank             int     `json:"rank"`
	Score            float64 `json:"score"`
	KeywordRank      int     `json:"keyword_rank,omitempty"`
	VectorRank       int     `json:"vector_rank,omitempty"`
}

type indexedSource struct {
	EntryID, MaterialID, ClassID int64
	Content                      string
}
type storedChunk struct {
	ID         int64
	Index      int
	Text       string
	Start, End int
}

func NewRetrievalService(config Config, store *Store) *RetrievalService {
	return &RetrievalService{store: store, models: NewModelClient(config), qdrant: NewQdrantClient(config), config: config}
}

func (service *RetrievalService) Initialize(ctx context.Context) error {
	if err := service.qdrant.EnsureCollection(ctx); err != nil {
		return err
	}
	return service.Backfill(ctx)
}

func (service *RetrievalService) Backfill(ctx context.Context) error {
	rows, err := service.store.DB.QueryContext(ctx, `SELECT k.id, k.material_id, k.class_id, k.content FROM knowledge_entries k JOIN materials m ON m.id=k.material_id LEFT JOIN knowledge_chunks c ON c.knowledge_entry_id=k.id WHERE m.deletion_pending=FALSE GROUP BY k.id, k.material_id, k.class_id, k.content HAVING COUNT(c.id)=0`)
	if err != nil {
		return err
	}
	defer rows.Close()
	sources := []indexedSource{}
	for rows.Next() {
		var source indexedSource
		if err := rows.Scan(&source.EntryID, &source.MaterialID, &source.ClassID, &source.Content); err != nil {
			return err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, source := range sources {
		if err := service.indexSource(ctx, source, ChunkOptions{Strategy: "auto"}); err != nil {
			// Source rows are intentionally preserved; failed chunks remain available for reindex.
			continue
		}
	}
	return nil
}

func (service *RetrievalService) IndexMaterial(ctx context.Context, materialID, classID int64, options ChunkOptions) error {
	var source indexedSource
	err := service.store.DB.QueryRowContext(ctx, `SELECT k.id, k.material_id, k.class_id, k.content FROM knowledge_entries k JOIN materials m ON m.id=k.material_id WHERE k.material_id=? AND k.class_id=? AND m.deletion_pending=FALSE`, materialID, classID).Scan(&source.EntryID, &source.MaterialID, &source.ClassID, &source.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return service.indexSource(ctx, source, options)
}

func (service *RetrievalService) DeleteMaterial(ctx context.Context, materialID, classID int64, uploadDir string) error {
	service.indexMu.Lock()
	defer service.indexMu.Unlock()
	storedFilename, err := service.store.StartMaterialDeletion(ctx, materialID, classID)
	if err != nil { return err }
	if err := service.qdrant.DeleteMaterialPoints(ctx, materialID, classID); err != nil { return err }
	path := filepath.Join(uploadDir, filepath.Base(storedFilename))
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) { return err }
	return service.store.CompleteMaterialDeletion(ctx, materialID, classID)
}


func (service *RetrievalService) indexSource(ctx context.Context, source indexedSource, options ChunkOptions) error {
	service.indexMu.Lock()
	defer service.indexMu.Unlock()
	drafts, err := SplitKnowledge(source.Content, options)
	if err != nil {
		return err
	}
	if len(drafts) == 0 {
		return fmt.Errorf("source text produced no chunks")
	}
	oldIDs, err := service.chunkIDs(ctx, source.EntryID)
	if err != nil {
		return err
	}
	if err := service.qdrant.Delete(ctx, oldIDs); err != nil {
		return err
	}
	tx, err := service.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_chunks WHERE knowledge_entry_id=?`, source.EntryID); err != nil {
		return err
	}
	chunks := make([]storedChunk, 0, len(drafts))
	for _, draft := range drafts {
		result, err := tx.ExecContext(ctx, `INSERT INTO knowledge_chunks (knowledge_entry_id, material_id, class_id, chunk_index, chunk_text, start_offset, end_offset, index_status) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending')`, source.EntryID, source.MaterialID, source.ClassID, draft.Index, draft.Text, draft.StartOffset, draft.EndOffset)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		chunks = append(chunks, storedChunk{ID: id, Index: draft.Index, Text: draft.Text, Start: draft.StartOffset, End: draft.EndOffset})
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	texts := make([]string, len(chunks))
	for index, chunk := range chunks {
		texts[index] = chunk.Text
	}
	vectors, err := service.models.Embed(ctx, texts)
	if err != nil {
		service.markChunksFailed(ctx, source.EntryID)
		return err
	}
	points := make([]VectorPoint, len(chunks))
	for index, chunk := range chunks {
		points[index] = VectorPoint{ID: chunk.ID, Vector: vectors[index], ClassID: source.ClassID, MaterialID: source.MaterialID, KnowledgeEntryID: source.EntryID, ChunkIndex: chunk.Index}
	}
	if err := service.qdrant.Upsert(ctx, points); err != nil {
		service.markChunksFailed(ctx, source.EntryID)
		return err
	}
	if _, err := service.store.DB.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='ready' WHERE knowledge_entry_id=? AND index_status='pending'`, source.EntryID); err != nil {
		_ = service.qdrant.Delete(ctx, chunkIDsFromStored(chunks))
		service.markChunksFailed(ctx, source.EntryID)
		return err
	}
	return nil
}

func (service *RetrievalService) markChunksFailed(ctx context.Context, entryID int64) {
	_, _ = service.store.DB.ExecContext(ctx, `UPDATE knowledge_chunks SET index_status='failed' WHERE knowledge_entry_id=? AND index_status='pending'`, entryID)
}
func chunkIDsFromStored(chunks []storedChunk) []int64 {
	result := make([]int64, len(chunks))
	for index, chunk := range chunks {
		result[index] = chunk.ID
	}
	return result
}
func (service *RetrievalService) chunkIDs(ctx context.Context, entryID int64) ([]int64, error) {
	rows, err := service.store.DB.QueryContext(ctx, `SELECT id FROM knowledge_chunks WHERE knowledge_entry_id=?`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func (service *RetrievalService) Search(ctx context.Context, classID int64, query, mode string, limit int) ([]SearchHit, error) {
	if mode == "" {
		mode = "hybrid"
	}
	if limit < 1 || limit > 20 {
		limit = 10
	}
	switch mode {
	case "keyword":
		return service.keyword(ctx, classID, query, limit)
	case "vector":
		return service.vector(ctx, classID, query, limit)
	case "hybrid":
		keyword, err := service.keyword(ctx, classID, query, 20)
		if err != nil {
			return nil, err
		}
		vector, err := service.vector(ctx, classID, query, 20)
		if err != nil {
			return nil, err
		}
		return fuseRRF(keyword, vector, limit), nil
	default:
		return nil, fmt.Errorf("unsupported search mode")
	}
}

func (service *RetrievalService) keyword(ctx context.Context, classID int64, query string, limit int) ([]SearchHit, error) {
	// FULLTEXT relevance is corpus-relative, so an exact literal match can score
	// below the configured floor even when the requested phrase is present.
	// Keep the threshold for ranked/soft matches, but do not discard exact hits.
	rows, err := service.store.DB.QueryContext(ctx, `SELECT c.id, c.knowledge_entry_id, c.material_id, c.chunk_index, c.start_offset, c.end_offset, c.chunk_text, m.title, MATCH(c.chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) AS relevance FROM knowledge_chunks c JOIN materials m ON m.id=c.material_id WHERE c.class_id=? AND m.deletion_pending=FALSE AND c.index_status='ready' AND (MATCH(c.chunk_text) AGAINST (? IN NATURAL LANGUAGE MODE) >= ? OR LOCATE(?, c.chunk_text) > 0) ORDER BY relevance DESC, c.id ASC LIMIT ?`, query, classID, query, service.config.KeywordMinScore, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []SearchHit{}
	for rows.Next() {
		var hit SearchHit
		if err := rows.Scan(&hit.ChunkID, &hit.KnowledgeEntryID, &hit.MaterialID, &hit.ChunkIndex, &hit.StartOffset, &hit.EndOffset, &hit.Excerpt, &hit.MaterialTitle, &hit.Score); err != nil {
			return nil, err
		}
		hit.Rank = len(result) + 1
		hit.KeywordRank = hit.Rank
		hit.MaterialURL = fmt.Sprintf("/api/materials/%d/file", hit.MaterialID)
		result = append(result, hit)
	}
	return result, rows.Err()
}

func (service *RetrievalService) vector(ctx context.Context, classID int64, query string, limit int) ([]SearchHit, error) {
	vectors, err := service.models.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	matches, err := service.qdrant.Search(ctx, vectors[0], classID, limit, service.config.VectorMinScore)
	if err != nil {
		return nil, err
	}
	result := []SearchHit{}
	for _, match := range matches {
		var hit SearchHit
		err := service.store.DB.QueryRowContext(ctx, `SELECT c.id,c.knowledge_entry_id,c.material_id,c.chunk_index,c.start_offset,c.end_offset,c.chunk_text,m.title FROM knowledge_chunks c JOIN materials m ON m.id=c.material_id WHERE c.id=? AND c.class_id=? AND m.deletion_pending=FALSE AND c.index_status='ready'`, match.ID, classID).Scan(&hit.ChunkID, &hit.KnowledgeEntryID, &hit.MaterialID, &hit.ChunkIndex, &hit.StartOffset, &hit.EndOffset, &hit.Excerpt, &hit.MaterialTitle)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		hit.Score = match.Score
		hit.Rank = len(result) + 1
		hit.VectorRank = hit.Rank
		hit.MaterialURL = fmt.Sprintf("/api/materials/%d/file", hit.MaterialID)
		result = append(result, hit)
	}
	return result, nil
}

func fuseRRF(keyword, vector []SearchHit, limit int) []SearchHit {
	type fused struct {
		hit   SearchHit
		score float64
		best  int
	}
	items := map[int64]*fused{}
	add := func(hits []SearchHit, isKeyword bool) {
		for index, hit := range hits {
			rank := index + 1
			item := items[hit.ChunkID]
			if item == nil {
				copy := hit
				copy.Score = 0
				copy.Rank = 0
				copy.KeywordRank = 0
				copy.VectorRank = 0
				item = &fused{hit: copy, best: rank}
				items[hit.ChunkID] = item
			}
			item.score += 1.0 / float64(60+rank)
			if rank < item.best {
				item.best = rank
			}
			if isKeyword {
				item.hit.KeywordRank = rank
			} else {
				item.hit.VectorRank = rank
			}
		}
	}
	add(keyword, true)
	add(vector, false)
	ordered := make([]*fused, 0, len(items))
	for _, item := range items {
		ordered = append(ordered, item)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].score != ordered[j].score {
			return ordered[i].score > ordered[j].score
		}
		if ordered[i].best != ordered[j].best {
			return ordered[i].best < ordered[j].best
		}
		return ordered[i].hit.ChunkID < ordered[j].hit.ChunkID
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	result := make([]SearchHit, len(ordered))
	for index, item := range ordered {
		item.hit.Score = item.score
		item.hit.Rank = index + 1
		result[index] = item.hit
	}
	return result
}

func buildEvidencePrompt(question string, hits []SearchHit) string {
	var builder strings.Builder
	for index, hit := range hits {
		fmt.Fprintf(&builder, "[%d] 材料：%s；chunk：%d\n%s\n", index+1, hit.MaterialTitle, hit.ChunkIndex, hit.Excerpt)
	}
	fmt.Fprintf(&builder, "问题：%s", question)
	return builder.String()
}
