package uploads

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"io"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func init() {
	sql.Register("fluffcatch_uploads_test", fakeDriver{})
}

func TestDetectUploadContentTypeSupportsISOImageBrands(t *testing.T) {
	tests := map[string]string{
		"heic": "image/heic",
		"heif": "image/heif",
		"avif": "image/avif",
	}

	for brand, expected := range tests {
		t.Run(brand, func(t *testing.T) {
			if actual := detectUploadContentType(testFTYP(brand)); actual != expected {
				t.Fatalf("expected %s, got %s", expected, actual)
			}
		})
	}
}

func TestDetectUploadContentTypeDoesNotTrustExtension(t *testing.T) {
	if actual := detectUploadContentType([]byte("not an image")); actual == "image/heic" || actual == "image/heif" || actual == "image/avif" {
		t.Fatalf("unexpected ISO image type for non-image data: %s", actual)
	}
}

func TestAdminApprovedUploadIgnoresSubmissionEnabled(t *testing.T) {
	sqlDB, err := sql.Open("fluffcatch_uploads_test", "")
	if err != nil {
		t.Fatalf("open fake sql db: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open fake mysql db: %v", err)
	}

	service := NewService(db, nil, 20)
	err = service.verifyEventExists(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected admin event check to ignore submission switch, got %v", err)
	}
	err = service.verifyEventAllowsSubmission(context.Background(), 1)
	if err == nil || err.Error() != "submissions are closed" {
		t.Fatalf("expected public submission check to reject closed submissions, got %v", err)
	}
}

func testFTYP(brand string) []byte {
	payload := append(append([]byte(brand), 0, 0, 0, 0), []byte("mif1"+brand)...)
	content := binary.BigEndian.AppendUint32(nil, uint32(len(payload)+8))
	content = append(content, []byte("ftyp")...)
	content = append(content, payload...)
	return content
}

type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) {
	return fakeSQLConn{}, nil
}

type fakeSQLConn struct{}

func (fakeSQLConn) Prepare(query string) (driver.Stmt, error) {
	return fakeStmt{}, nil
}

func (fakeSQLConn) Close() error {
	return nil
}

func (fakeSQLConn) Begin() (driver.Tx, error) {
	return fakeTx{}, nil
}

func (fakeSQLConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return &fakeRows{columns: []string{"id", "submission_enabled"}, values: [][]driver.Value{{int64(1), false}}}, nil
}

func (fakeSQLConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return fakeResult(0), nil
}

type fakeStmt struct{}

func (fakeStmt) Close() error {
	return nil
}

func (fakeStmt) NumInput() int {
	return -1
}

func (fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return fakeResult(0), nil
}

func (fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &fakeRows{columns: []string{"id", "submission_enabled"}, values: [][]driver.Value{{int64(1), false}}}, nil
}

type fakeTx struct{}

func (fakeTx) Commit() error {
	return nil
}

func (fakeTx) Rollback() error {
	return nil
}

type fakeResult int64

func (result fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (result fakeResult) RowsAffected() (int64, error) {
	return int64(result), nil
}

type fakeRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (rows fakeRows) Columns() []string {
	return rows.columns
}

func (rows fakeRows) Close() error {
	return nil
}

func (rows *fakeRows) Next(dest []driver.Value) error {
	if rows.index >= len(rows.values) {
		return io.EOF
	}
	copy(dest, rows.values[rows.index])
	rows.index++
	return nil
}
