package db

import "testing"

func TestMigrateCreatesTables(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, tbl := range []string{"users", "books", "characters", "scenes", "chapters", "panels"} {
		var name string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err != nil {
			t.Fatalf("表 %s 不存在: %v", tbl, err)
		}
	}
}
