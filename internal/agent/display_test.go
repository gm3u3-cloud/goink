package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/sigpanic/goink/internal/chapter"
	"github.com/sigpanic/goink/internal/mcp_tools"
	"github.com/sigpanic/goink/internal/volume"
)

func TestBuildDisplay_UsesChapterIDAndReadingNumber(t *testing.T) {
	db := newDisplayTestDB(t)
	ctx := context.Background()
	requireCreate(t, db.Create(&volume.Volume{ID: 8, NovelID: 1, Name: "第一卷", SortOrder: 1}))
	volumeID := int64(8)
	requireCreate(t, db.Create(&chapter.Chapter{ID: 42, NovelID: 1, VolumeID: &volumeID, SortOrder: 1, Title: "开篇"}))
	requireCreate(t, db.Create(&chapter.Chapter{ID: 7, NovelID: 1, VolumeID: &volumeID, SortOrder: 2, Title: "后续"}))

	a := &Agent{chapterStore: chapter.NewStore(db, slog.New(slog.NewTextHandler(io.Discard, nil)))}

	got := a.buildDisplay(ctx, "read", map[string]any{"path": "chapters/id_42.md"}, mcp_tools.PhaseCompleted, 1)
	if got.DisplayText != "Просмотр Глава 1 开篇" {
		t.Errorf("display text = %q, want %q", got.DisplayText, "Просмотр Глава 1 开篇")
	}

	got = a.buildDisplay(ctx, "edit", map[string]any{"chapter_id": float64(7)}, mcp_tools.PhaseCompleted, 1)
	if got.DisplayText != "Редактирование Глава 2 后续" {
		t.Errorf("display text = %q, want %q", got.DisplayText, "Редактирование Глава 2 后续")
	}

	got = a.buildDisplay(ctx, "read", map[string]any{"path": "outlines/8/id_42.md"}, mcp_tools.PhaseCompleted, 1)
	if got.DisplayText != "Просмотр Глава 1 开篇план" {
		t.Errorf("display text = %q, want %q", got.DisplayText, "Просмотр Глава 1 开篇план")
	}
}

func TestBuildDisplay_ChapterLookupFallbackUsesID(t *testing.T) {
	a := &Agent{chapterStore: chapter.NewStore(newDisplayTestDB(t), slog.New(slog.NewTextHandler(io.Discard, nil)))}

	got := a.buildDisplay(context.Background(), "read", map[string]any{"chapter_id": int64(42)}, mcp_tools.PhaseCompleted, 1)
	if got.DisplayText != "Просмотр Глава (ID: 42)" {
		t.Errorf("display text = %q, want %q", got.DisplayText, "Просмотр Глава (ID: 42)")
	}
}

func TestChapterID(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want int64
		ok   bool
	}{
		{name: "argument", args: map[string]any{"chapter_id": float64(42)}, want: 42, ok: true},
		{name: "path", args: map[string]any{"path": "chapters/id_42.md"}, want: 42, ok: true},
		{name: "volume path", args: map[string]any{"path": "outlines/8/id_42.md"}, want: 42, ok: true},
		{name: "legacy number ignored", args: map[string]any{"chapter_number": float64(42)}, ok: false},
		{name: "legacy path ignored", args: map[string]any{"path": "chapters/042.md"}, ok: false},
		{name: "fraction rejected", args: map[string]any{"chapter_id": 42.5}, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := chapterID(tt.args)
			if got != tt.want || ok != tt.ok {
				t.Errorf("chapterID() = (%d, %t), want (%d, %t)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestBuildDisplayRunMCP(t *testing.T) {
	a := &Agent{}
	for _, tt := range []struct {
		target string
		phase  mcp_tools.DisplayPhase
		want   string
	}{
		{"get_volumes", mcp_tools.PhaseExecuting, "Идёт Список томов"},
		{"get_volumes", mcp_tools.PhaseCompleted, "Список томов"},
		{"get_volumes", mcp_tools.PhaseFailed, "Список томов"},
		{"place_volume", mcp_tools.PhaseCompleted, "Создать или переместить том"},
		{"update_volume", mcp_tools.PhaseCompleted, "Переименовать том"},
		{"unknown_extension", mcp_tools.PhaseCompleted, "Вызов unknown_extension"},
		{"run_mcp", mcp_tools.PhaseFailed, "Вызов инструмента по запросу"},
		{"", mcp_tools.PhaseSelected, "Идёт Вызов инструмента по запросу"},
	} {
		got := a.buildDisplay(context.Background(), "run_mcp", map[string]any{"tool_name": tt.target}, tt.phase, 1)
		if got.DisplayText != tt.want {
			t.Errorf("target %q: got %q, want %q", tt.target, got.DisplayText, tt.want)
		}
		if tt.target != "" && got.Metadata["tool_name"] != tt.target {
			t.Errorf("target %q missing from metadata: %v", tt.target, got.Metadata)
		}
	}
}

func newDisplayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&volume.Volume{}, &chapter.Chapter{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return db
}

func requireCreate(t *testing.T, result *gorm.DB) {
	t.Helper()
	if result.Error != nil {
		t.Fatalf("create record: %v", result.Error)
	}
}
