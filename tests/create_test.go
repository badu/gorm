package tests

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func TestCreate(t *testing.T) {
	t.Log("47) TestCreate")
	float := 35.03554004971999
	now := time.Now()
	user := User{Name: "CreateUser", Age: 18, Birthday: &now, UserNum: Num(111), PasswordHash: []byte{'f', 'a', 'k', '4'}, Latitude: float}

	if !TestDB.NewRecord(user) || !TestDB.NewRecord(&user) {
		t.Error("User should be new record before create")
	}

	if count := TestDB.Save(&user).RowsAffected; count != 1 {
		t.Error("There should be one record be affected when create record")
	}

	if TestDB.NewRecord(user) || TestDB.NewRecord(&user) {
		t.Error("User should not new record after save")
	}

	var newUser User
	TestDB.First(&newUser, user.Id)

	if !reflect.DeepEqual(newUser.PasswordHash, []byte{'f', 'a', 'k', '4'}) {
		t.Errorf("User's PasswordHash should be saved ([]byte)")
	}

	if newUser.Age != 18 {
		t.Errorf("User's Age should be saved (int)")
	}

	if newUser.UserNum != Num(111) {
		t.Errorf("User's UserNum should be saved (custom type)")
	}

	if newUser.Latitude != float {
		t.Errorf("Float64 should not be changed after save")
	}

	if user.CreatedAt.IsZero() {
		t.Errorf("Should have created_at after create")
	}

	if newUser.CreatedAt.IsZero() {
		t.Errorf("Should have created_at after create")
	}

	TestDB.Model(user).Update("name", "create_user_new_name")
	TestDB.First(&user, user.Id)
	if user.CreatedAt.Format(time.RFC3339Nano) != newUser.CreatedAt.Format(time.RFC3339Nano) {
		t.Errorf("CreatedAt should not be changed after update")
	}
}

func TestCreateWithAutoIncrement(t *testing.T) {
	t.Log("48) TestCreateWithAutoIncrement")
	if dialect := os.Getenv("GORM_DIALECT"); dialect != "postgres" {
		t.Skip("Skipping this because only postgres properly support auto_increment on a non-primary_key column")
	}
	user1 := User{}
	user2 := User{}

	TestDB.Create(&user1)
	TestDB.Create(&user2)

	if user2.Sequence-user1.Sequence != 1 {
		t.Errorf("Auto increment should apply on Sequence")
	}
}

func TestCreateWithNoGORMPrimayKey(t *testing.T) {
	t.Log("49) TestCreateWithNoGORMPrimayKey")
	if dialect := os.Getenv("GORM_DIALECT"); dialect == "mssql" {
		t.Skip("Skipping this because MSSQL will return identity only if the table has an Id column")
	}

	jt := JoinTable{From: 1, To: 2}
	err := TestDB.Create(&jt).Error
	if err != nil {
		t.Errorf("No error should happen when create a record without a GORM primary key. But in the database this primary key exists and is the union of 2 or more fields\n But got: %s", err)
	}
}

func TestCreateWithNoStdPrimaryKeyAndDefaultValues(t *testing.T) {
	t.Log("50) TestCreateWithNoStdPrimaryKeyAndDefaultValues")
	animal := Animal{Name: "Ferdinand"}
	if TestDB.Save(&animal).Error != nil {
		t.Errorf("No error should happen when create a record without std primary key")
	}

	if animal.Counter == 0 {
		t.Errorf("No std primary key should be filled value after create")
	}

	if animal.Name != "Ferdinand" {
		t.Errorf("Default value should be overrided")
	}

	// Test create with default value not overrided
	an := Animal{From: "nerdz"}

	if TestDB.Save(&an).Error != nil {
		t.Errorf("No error should happen when create an record without std primary key")
	}

	// We must fetch the value again, to have the default fields updated
	// (We can't do this in the update statements, since sql default can be expressions
	// And be different from the fields' type (eg. a time.Time fields has a default value of "now()"
	TestDB.Model(Animal{}).Where(&Animal{Counter: an.Counter}).First(&an)

	if an.Name != "galeone" {
		t.Errorf("Default value should fill the field. But got %v", an.Name)
	}
}

func TestAnonymousScanner(t *testing.T) {
	t.Log("51) TestAnonymousScanner")
	user := User{Name: "anonymous_scanner", Role: Role{Name: "admin"}}
	TestDB.Save(&user)

	var user2 User
	TestDB.First(&user2, "name = ?", "anonymous_scanner")
	if user2.Role.Name != "admin" {
		t.Errorf("Should be able to get anonymous scanner")
	}

	if !user2.IsAdmin() {
		t.Errorf("Should be able to get anonymous scanner")
	}
}

func TestAnonymousField(t *testing.T) {
	t.Log("52) TestAnonymousField")
	user := User{Name: "anonymous_field", Company: Company{Name: "company"}}
	TestDB.Save(&user)

	var user2 User
	TestDB.First(&user2, "name = ?", "anonymous_field")
	TestDB.Model(&user2).Related(&user2.Company)
	if user2.Company.Name != "company" {
		t.Errorf("Should be able to get anonymous field")
	}
}

func TestSelectWithCreate(t *testing.T) {
	t.Log("53) TestSelectWithCreate")
	user := getPreparedUser("select_user", "select_with_create")
	TestDB.Select("Name", "BillingAddress", "CreditCard", "Company", "Emails").Create(user)

	var queryuser User
	TestDB.Preload("BillingAddress").Preload("ShippingAddress").
		Preload("CreditCard").Preload("Emails").Preload("Company").First(&queryuser, user.Id)

	if queryuser.Name != user.Name || queryuser.Age == user.Age {
		t.Errorf("Should only create users with name column")
	}

	if queryuser.BillingAddressID.Int64 == 0 || queryuser.ShippingAddressId != 0 ||
		queryuser.CreditCard.ID == 0 || len(queryuser.Emails) == 0 {
		t.Errorf("Should only create selected relationships")
	}
}

func TestOmitWithCreate(t *testing.T) {
	t.Log("54) TestOmitWithCreate")
	user := getPreparedUser("omit_user", "omit_with_create")
	TestDB.Omit("Name", "BillingAddress", "CreditCard", "Company", "Emails").Create(user)

	var queryuser User
	TestDB.Preload("BillingAddress").Preload("ShippingAddress").
		Preload("CreditCard").Preload("Emails").Preload("Company").First(&queryuser, user.Id)

	if queryuser.Name == user.Name || queryuser.Age != user.Age {
		t.Errorf("Should only create users with age column")
	}

	if queryuser.BillingAddressID.Int64 != 0 || queryuser.ShippingAddressId == 0 ||
		queryuser.CreditCard.ID != 0 || len(queryuser.Emails) != 0 {
		t.Errorf("Should not create omited relationships")
	}
}
