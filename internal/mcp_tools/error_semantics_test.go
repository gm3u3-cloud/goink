package mcp_tools

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/sigpanic/goink/internal/llm"
	"github.com/sigpanic/goink/internal/preference"
	"github.com/sigpanic/goink/internal/setting"
)

func testMCPRegistry(t *testing.T, tool Tool) *Registry {
	t.Helper()
	reg := NewRegistry(slog.New(slog.NewTextHandler(io.Discard, nil)))
	reg.Register(tool)
	return reg
}

func testMCPDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWebSearchRequestFailureIsSystemError(t *testing.T) {
	reg := testMCPRegistry(t, &WebSearchTool{})
	result := reg.Execute(context.Background(), "web_search", []byte(`{"prompt":"查询天气"}`), ToolContext{
		WebSearch: func(context.Context, string) (*llm.WebSearchResult, error) {
			return nil, errors.New("request timed out")
		},
	}, nil)

	if result.Success {
		t.Fatal("expected failure")
	}
	if result.ErrKind != ErrKindSystem {
		t.Errorf("ErrKind = %q, want %q", result.ErrKind, ErrKindSystem)
	}
	if !strings.Contains(result.Error, "ошибка поиска: request timed out") {
		t.Errorf("Error = %q, want search failure detail", result.Error)
	}
}

func TestUpsertPreferenceSeparatesBusinessAndSystemFailures(t *testing.T) {
	reg := testMCPRegistry(t, &UpsertPreferenceTool{})

	businessDB := testMCPDB(t)
	if err := businessDB.AutoMigrate(&preference.PreferenceItem{}); err != nil {
		t.Fatal(err)
	}
	business := reg.Execute(context.Background(), "upsert_preference", []byte(`{"preferences":[{"preference_id":99,"category":"文风","content":"简洁"}]}`), ToolContext{DB: businessDB, NovelID: 1}, nil)
	if business.Success || business.ErrKind != ErrKindBusiness {
		t.Errorf("business result = %+v, want business failure", business)
	}
	if !strings.Contains(business.Error, "предпочтение 99 не существует") {
		t.Errorf("business error = %q", business.Error)
	}

	system := reg.Execute(context.Background(), "upsert_preference", []byte(`{"preferences":[{"category":"文风","content":"简洁"}]}`), ToolContext{DB: testMCPDB(t), NovelID: 1}, nil)
	if system.Success || system.ErrKind != ErrKindSystem {
		t.Errorf("system result = %+v, want system failure", system)
	}
	if !strings.Contains(system.Error, "upsert предпочтения, запись 0 [文风] (транзакция откатлена)") {
		t.Errorf("system error = %q, want rollback state", system.Error)
	}
}

func TestUpsertSettingSeparatesBusinessAndSystemFailures(t *testing.T) {
	reg := testMCPRegistry(t, &UpsertSettingTool{})

	businessDB := testMCPDB(t)
	if err := businessDB.AutoMigrate(&setting.SettingItem{}); err != nil {
		t.Fatal(err)
	}
	business := reg.Execute(context.Background(), "upsert_setting", []byte(`{"settings":[{"setting_id":99,"category":"世界观","content":"灵气"}]}`), ToolContext{DB: businessDB, NovelID: 1}, nil)
	if business.Success || business.ErrKind != ErrKindBusiness {
		t.Errorf("business result = %+v, want business failure", business)
	}
	if !strings.Contains(business.Error, "настройка 99 не существует") {
		t.Errorf("business error = %q", business.Error)
	}

	system := reg.Execute(context.Background(), "upsert_setting", []byte(`{"settings":[{"category":"世界观","content":"灵气"}]}`), ToolContext{DB: testMCPDB(t), NovelID: 1}, nil)
	if system.Success || system.ErrKind != ErrKindSystem {
		t.Errorf("system result = %+v, want system failure", system)
	}
	if !strings.Contains(system.Error, "upsert настройки, запись 0 [世界观] (транзакция откатлена)") {
		t.Errorf("system error = %q, want rollback state", system.Error)
	}
}
