package tests

import (
	"fmt"
	"testing"
	"time"
)

func TestIndexes(t *testing.T) {
	t.Log("69) TestIndexes")
	if err := TestDB.Model(&Email{}).AddIndex("idx_email_email", "email").Error; err != nil {
		t.Errorf("Got error when tried to create index: %+v", err)
	}

	scope := TestDB.NewScope(&Email{})
	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email") {
		t.Errorf("Email should have index idx_email_email")
	}

	if err := TestDB.Model(&Email{}).RemoveIndex("idx_email_email").Error; err != nil {
		t.Errorf("Got error when tried to remove index: %+v", err)
	}

	if TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email") {
		t.Errorf("Email's index idx_email_email should be deleted")
	}

	if err := TestDB.Model(&Email{}).AddIndex("idx_email_email_and_user_id", "user_id", "email").Error; err != nil {
		t.Errorf("Got error when tried to create index: %+v", err)
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email_and_user_id") {
		t.Errorf("Email should have index idx_email_email_and_user_id")
	}

	if err := TestDB.Model(&Email{}).RemoveIndex("idx_email_email_and_user_id").Error; err != nil {
		t.Errorf("Got error when tried to remove index: %+v", err)
	}

	if TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email_and_user_id") {
		t.Errorf("Email's index idx_email_email_and_user_id should be deleted")
	}

	if err := TestDB.Model(&Email{}).AddUniqueIndex("idx_email_email_and_user_id", "user_id", "email").Error; err != nil {
		t.Errorf("Got error when tried to create index: %+v", err)
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email_and_user_id") {
		t.Errorf("Email should have index idx_email_email_and_user_id")
	}

	if TestDB.Save(&User{Name: "unique_indexes", Emails: []Email{{Email: "user1@example.comiii"}, {Email: "user1@example.com"}, {Email: "user1@example.com"}}}).Error == nil {
		t.Errorf("Should get to create duplicate record when having unique index")
	}

	var user = User{Name: "sample_user"}
	TestDB.Save(&user)
	if TestDB.Model(&user).Association("Emails").Append(Email{Email: "not-1duplicated@gmail.com"}, Email{Email: "not-duplicated2@gmail.com"}).Error != nil {
		t.Errorf("Should get no error when append two emails for user")
	}

	if TestDB.Model(&user).Association("Emails").Append(Email{Email: "duplicated@gmail.com"}, Email{Email: "duplicated@gmail.com"}).Error == nil {
		t.Errorf("Should get no duplicated email error when insert duplicated emails for a user")
	}

	if err := TestDB.Model(&Email{}).RemoveIndex("idx_email_email_and_user_id").Error; err != nil {
		t.Errorf("Got error when tried to remove index: %+v", err)
	}

	if TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_email_and_user_id") {
		t.Errorf("Email's index idx_email_email_and_user_id should be deleted")
	}

	if TestDB.Save(&User{Name: "unique_indexes", Emails: []Email{{Email: "user1@example.com"}, {Email: "user1@example.com"}}}).Error != nil {
		t.Errorf("Should be able to create duplicated emails after remove unique index")
	}
}

func TestAutoMigration(t *testing.T) {
	t.Log("70) TestAutoMigration")
	TestDB.AutoMigrate(&Address{})
	if err := TestDB.Table("emails").AutoMigrate(&BigEmail{}).Error; err != nil {
		t.Errorf("Auto Migrate should not raise any error")
	}

	now := time.Now()
	TestDB.Save(&BigEmail{Email: "jinzhu@example.org", UserAgent: "pc", RegisteredAt: &now})

	scope := TestDB.NewScope(&BigEmail{})
	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_email_agent") {
		t.Errorf("Failed to create index")
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "uix_emails_registered_at") {
		t.Errorf("Failed to create index")
	}

	var bigemail BigEmail
	TestDB.First(&bigemail, "user_agent = ?", "pc")
	if bigemail.Email != "jinzhu@example.org" || bigemail.UserAgent != "pc" || bigemail.RegisteredAt.IsZero() {
		t.Error("Big Emails should be saved and fetched correctly")
	}
}

func TestMultipleIndexes(t *testing.T) {
	t.Log("71) TestMultipleIndexes")
	if err := TestDB.DropTableIfExists(&MultipleIndexes{}).Error; err != nil {
		fmt.Printf("Got error when try to delete table multiple_indexes, %+v\n", err)
	}

	TestDB.AutoMigrate(&MultipleIndexes{})
	if err := TestDB.AutoMigrate(&BigEmail{}).Error; err != nil {
		t.Errorf("Auto Migrate should not raise any error")
	}

	TestDB.Save(&MultipleIndexes{UserID: 1, Name: "jinzhu", Email: "jinzhu@example.org", Other: "foo"})

	scope := TestDB.NewScope(&MultipleIndexes{})
	if !TestDB.Dialect().HasIndex(scope.TableName(), "uix_multipleindexes_user_name") {
		t.Errorf("Failed to create index")
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "uix_multipleindexes_user_email") {
		t.Errorf("Failed to create index")
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "uix_multiple_indexes_email") {
		t.Errorf("Failed to create index")
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_multipleindexes_user_other") {
		t.Errorf("Failed to create index")
	}

	if !TestDB.Dialect().HasIndex(scope.TableName(), "idx_multiple_indexes_other") {
		t.Errorf("Failed to create index")
	}

	var mutipleIndexes MultipleIndexes
	TestDB.First(&mutipleIndexes, "name = ?", "jinzhu")
	if mutipleIndexes.Email != "jinzhu@example.org" || mutipleIndexes.Name != "jinzhu" {
		t.Error("MutipleIndexes should be saved and fetched correctly")
	}

	// Check unique constraints
	if err := TestDB.Save(&MultipleIndexes{UserID: 1, Name: "name1", Email: "jinzhu@example.org", Other: "foo"}).Error; err == nil {
		t.Error("MultipleIndexes unique index failed")
	}

	if err := TestDB.Save(&MultipleIndexes{UserID: 1, Name: "name1", Email: "foo@example.org", Other: "foo"}).Error; err != nil {
		t.Error("MultipleIndexes unique index failed")
	}

	if err := TestDB.Save(&MultipleIndexes{UserID: 2, Name: "name1", Email: "jinzhu@example.org", Other: "foo"}).Error; err == nil {
		t.Error("MultipleIndexes unique index failed")
	}

	if err := TestDB.Save(&MultipleIndexes{UserID: 2, Name: "name1", Email: "foo2@example.org", Other: "foo"}).Error; err != nil {
		t.Error("MultipleIndexes unique index failed")
	}
}
