package repositories

import (
	"testing"

	"evo-bot-go/internal/testutils"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	promptSelectSQL = `SELECT template_text FROM prompting_templates WHERE template_key = $1`
	promptInsertSQL = `INSERT INTO prompting_templates (template_key, template_text) VALUES ($1, $2) ON CONFLICT (template_key) DO NOTHING`
)

func TestPromptingTemplateRepository_Get_Found(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewPromptingTemplateRepository(db)

	mock.ExpectQuery(exactSQL(promptSelectSQL)).WithArgs("summary_prompt").
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}).AddRow("stored prompt"))

	text, err := repo.Get("summary_prompt", "default prompt")

	require.NoError(t, err)
	assert.Equal(t, "stored prompt", text)
}

func TestPromptingTemplateRepository_Get_NotFoundInsertsDefault(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewPromptingTemplateRepository(db)

	mock.ExpectQuery(exactSQL(promptSelectSQL)).WithArgs("summary_prompt").
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}))
	mock.ExpectExec(exactSQL(promptInsertSQL)).WithArgs("summary_prompt", "default prompt").
		WillReturnResult(sqlmock.NewResult(0, 1))

	text, err := repo.Get("summary_prompt", "default prompt")

	require.NoError(t, err)
	assert.Equal(t, "default prompt", text)
}

func TestPromptingTemplateRepository_Get_NotFoundInsertError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewPromptingTemplateRepository(db)

	mock.ExpectQuery(exactSQL(promptSelectSQL)).WithArgs("summary_prompt").
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}))
	mock.ExpectExec(exactSQL(promptInsertSQL)).WithArgs("summary_prompt", "default prompt").
		WillReturnError(errDB)

	text, err := repo.Get("summary_prompt", "default prompt")

	require.Error(t, err)
	assert.Empty(t, text)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to insert default prompting template")
}

func TestPromptingTemplateRepository_Get_QueryErrorDoesNotInsert(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewPromptingTemplateRepository(db)

	// No Exec expectation: an unexpected INSERT would fail the test
	mock.ExpectQuery(exactSQL(promptSelectSQL)).WithArgs("summary_prompt").WillReturnError(errDB)

	text, err := repo.Get("summary_prompt", "default prompt")

	require.Error(t, err)
	assert.Empty(t, text)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "failed to get prompting template")
}

func TestPromptingTemplateRepository_Get_ScanError(t *testing.T) {
	db, mock := testutils.NewMockDB(t)
	repo := NewPromptingTemplateRepository(db)

	mock.ExpectQuery(exactSQL(promptSelectSQL)).WithArgs("summary_prompt").
		WillReturnRows(sqlmock.NewRows([]string{"template_text"}).AddRow(nil))

	text, err := repo.Get("summary_prompt", "default prompt")

	require.Error(t, err)
	assert.Empty(t, text)
	assert.Contains(t, err.Error(), "failed to get prompting template")
}
