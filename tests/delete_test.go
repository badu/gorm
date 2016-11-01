package tests

import (
	"testing"
	"time"
)

func TestDelete(t *testing.T) {
	t.Log("62) TestDelete")
	user1, user2 := User{Name: "delete1"}, User{Name: "delete2"}
	TestDB.Save(&user1)
	TestDB.Save(&user2)

	if err := TestDB.Delete(&user1).Error; err != nil {
		t.Errorf("No error should happen when delete a record, err=%s", err)
	}

	if !TestDB.Where("name = ?", user1.Name).First(&User{}).RecordNotFound() {
		t.Errorf("User can't be found after delete")
	}

	if TestDB.Where("name = ?", user2.Name).First(&User{}).RecordNotFound() {
		t.Errorf("Other users that not deleted should be found-able")
	}
}

func TestInlineDelete(t *testing.T) {
	t.Log("63) TestInlineDelete")
	user1, user2 := User{Name: "inline_delete1"}, User{Name: "inline_delete2"}
	TestDB.Save(&user1)
	TestDB.Save(&user2)

	if TestDB.Delete(&User{}, user1.Id).Error != nil {
		t.Errorf("No error should happen when delete a record")
	} else if !TestDB.Where("name = ?", user1.Name).First(&User{}).RecordNotFound() {
		t.Errorf("User can't be found after delete")
	}

	if err := TestDB.Delete(&User{}, "name = ?", user2.Name).Error; err != nil {
		t.Errorf("No error should happen when delete a record, err=%s", err)
	} else if !TestDB.Where("name = ?", user2.Name).First(&User{}).RecordNotFound() {
		t.Errorf("User can't be found after delete")
	}
}

func TestSoftDelete(t *testing.T) {
	t.Log("64) TestSoftDelete")
	type User struct {
		Id        int64
		Name      string
		DeletedAt *time.Time
	}
	TestDB.AutoMigrate(&User{})

	user := User{Name: "soft_delete"}
	TestDB.Save(&user)
	TestDB.Delete(&user)

	if TestDB.First(&User{}, "name = ?", user.Name).Error == nil {
		t.Errorf("Can't find a soft deleted record")
	}

	if err := TestDB.Unscoped().First(&User{}, "name = ?", user.Name).Error; err != nil {
		t.Errorf("Should be able to find soft deleted record with Unscoped, but err=%s", err)
	}

	TestDB.Unscoped().Delete(&user)
	if !TestDB.Unscoped().First(&User{}, "name = ?", user.Name).RecordNotFound() {
		t.Errorf("Can't find permanently deleted record")
	}
}
