package tests

import (
	"fmt"
	"reflect"

	. "github.com/badu/reGorm"
	"testing"
	"time"
)

func FirstAndLast(t *testing.T) {
	TestDB.Save(&User{Name: "user1", Emails: []Email{{Email: "user1@example.com"}}})
	TestDB.Save(&User{Name: "user2", Emails: []Email{{Email: "user2@example.com"}}})

	var user1, user2, user3, user4 User
	TestDB.First(&user1)
	TestDB.Order("id").Limit(1).Find(&user2)

	TestDB.Last(&user3)
	TestDB.Order("id desc").Limit(1).Find(&user4)
	//TODO : @Badu - simplify
	if user1.Id != user2.Id || user3.Id != user4.Id {
		t.Errorf("First and Last should by order by primary key")
	}

	var users []User
	TestDB.First(&users)
	if len(users) != 1 {
		t.Errorf("Find first record as slice")
	}

	var user User
	if TestDB.Joins("left join emails on emails.user_id = users.id").First(&user).Error != nil {
		t.Errorf("Should not raise any error when order with Join table")
	}

	if user.Email != "" {
		t.Errorf("User's Email should be blank as no one set it")
	}
}

func FirstAndLastWithNoStdPrimaryKey(t *testing.T) {
	TestDB.Save(&Animal{Name: "animal1"})
	TestDB.Save(&Animal{Name: "animal2"})

	var animal1, animal2, animal3, animal4 Animal
	TestDB.First(&animal1)
	TestDB.Order("counter").Limit(1).Find(&animal2)

	TestDB.Last(&animal3)
	TestDB.Order("counter desc").Limit(1).Find(&animal4)
	//TODO : @Badu - simplify
	if animal1.Counter != animal2.Counter || animal3.Counter != animal4.Counter {
		t.Errorf("First and Last should work correctly")
	}
}

func FirstAndLastWithRaw(t *testing.T) {
	user1 := User{Name: "user", Emails: []Email{{Email: "user1@example.com"}}}
	user2 := User{Name: "user", Emails: []Email{{Email: "user2@example.com"}}}
	TestDB.Save(&user1)
	TestDB.Save(&user2)

	var user3, user4 User
	TestDB.Raw("select * from users WHERE name = ?", "user").First(&user3)
	if user3.Id != user1.Id {
		t.Errorf("Find first record with raw")
	}

	TestDB.Raw("select * from users WHERE name = ?", "user").Last(&user4)
	if user4.Id != user2.Id {
		t.Errorf("Find last record with raw")
	}
}

func UIntPrimaryKey(t *testing.T) {
	var animal Animal
	TestDB.First(&animal, uint64(1))
	if animal.Counter != 1 {
		t.Errorf("Fetch a record from with a non-int primary key should work, but failed")
	}

	TestDB.Model(Animal{}).Where(Animal{Counter: uint64(2)}).Scan(&animal)
	if animal.Counter != 2 {
		t.Errorf("Fetch a record from with a non-int primary key should work, but failed")
	}
}

func StringPrimaryKeyForNumericValueStartingWithZero(t *testing.T) {
	type AddressByZipCode struct {
		ZipCode string `gorm:"primary_key"`
		Address string
	}
	TestDB.DropTable(AddressByZipCode{})
	TestDB.AutoMigrate(&AddressByZipCode{})
	TestDB.Create(&AddressByZipCode{ZipCode: "00501", Address: "Holtsville"})

	var address AddressByZipCode
	TestDB.First(&address, "00501")
	if address.ZipCode != "00501" {
		t.Errorf("Fetch a record from with a string primary key for a numeric value starting with zero should work, but failed")
	}
}

func FindAsSliceOfPointers(t *testing.T) {
	TestDB.Save(&User{Name: "user"})

	var users []User
	TestDB.Find(&users)

	var userPointers []*User
	TestDB.Find(&userPointers)

	if len(users) == 0 || len(users) != len(userPointers) {
		t.Errorf("Find slice of pointers")
	}
}

func SearchWithPlainSQL(t *testing.T) {
	user1 := User{Name: "PlainSqlUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "PlainSqlUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "PlainSqlUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)
	scopedb := TestDB.Where("name LIKE ?", "%PlainSqlUser%")

	if TestDB.Where("name = ?", user1.Name).First(&User{}).RecordNotFound() {
		t.Errorf("Search with plain SQL")
	}

	if TestDB.Where("name LIKE ?", "%"+user1.Name+"%").First(&User{}).RecordNotFound() {
		t.Errorf("Search with plan SQL (regexp)")
	}

	var users []User
	TestDB.Find(&users, "name LIKE ? and age > ?", "%PlainSqlUser%", 1)
	if len(users) != 2 {
		t.Errorf("Should found 2 users that age > 1, but got %v", len(users))
	}

	TestDB.Where("name LIKE ?", "%PlainSqlUser%").Where("age >= ?", 1).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should found 3 users that age >= 1, but got %v", len(users))
	}

	scopedb.Where("age <> ?", 20).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should found 2 users age != 20, but got %v", len(users))
	}

	scopedb.Where("birthday > ?", parseTime("2000-1-1")).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should found 2 users's birthday > 2000-1-1, but got %v", len(users))
	}

	scopedb.Where("birthday > ?", "2002-10-10").Find(&users)
	if len(users) != 2 {
		t.Errorf("Should found 2 users's birthday >= 2002-10-10, but got %v", len(users))
	}

	scopedb.Where("birthday >= ?", "2010-1-1").Where("birthday < ?", "2020-1-1").Find(&users)
	if len(users) != 1 {
		t.Errorf("Should found 1 users's birthday < 2020-1-1 and >= 2010-1-1, but got %v", len(users))
	}

	TestDB.Where("name in (?)", []string{user1.Name, user2.Name}).Find(&users)
	if len(users) != 2 {
		t.Errorf("Should found 2 users, but got %v", len(users))
	}

	TestDB.Where("id in (?)", []int64{user1.Id, user2.Id, user3.Id}).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should found 3 users, but got %v", len(users))
	}

	TestDB.Where("id in (?)", user1.Id).Find(&users)
	if len(users) != 1 {
		t.Errorf("Should found 1 users, but got %v", len(users))
	}

	if err := TestDB.Where("id IN (?)", []string{}).Find(&users).Error; err != nil {
		t.Error("no error should happen when query with empty slice, but got: ", err)
	}

	if err := TestDB.Not("id IN (?)", []string{}).Find(&users).Error; err != nil {
		t.Error("no error should happen when query with empty slice, but got: ", err)
	}

	if TestDB.Where("name = ?", "none existing").Find(&[]User{}).RecordNotFound() {
		t.Errorf("Should not get RecordNotFound error when looking for none existing records")
	}
}

func SearchWithStruct(t *testing.T) {
	user1 := User{Name: "StructSearchUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "StructSearchUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "StructSearchUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	if TestDB.Where(user1.Id).First(&User{}).RecordNotFound() {
		t.Errorf("Search with primary key")
	}

	if TestDB.First(&User{}, user1.Id).RecordNotFound() {
		t.Errorf("Search with primary key as inline condition")
	}

	if TestDB.First(&User{}, fmt.Sprintf("%v", user1.Id)).RecordNotFound() {
		t.Errorf("Search with primary key as inline condition")
	}

	var users []User
	TestDB.Where([]int64{user1.Id, user2.Id, user3.Id}).Find(&users)
	if len(users) != 3 {
		t.Errorf("Should found 3 users when search with primary keys, but got %v", len(users))
	}

	var user User
	TestDB.First(&user, &User{Name: user1.Name})
	if user.Id == 0 || user.Name != user1.Name {
		t.Errorf("Search first record with inline pointer of struct")
	}

	TestDB.First(&user, User{Name: user1.Name})
	if user.Id == 0 || user.Name != user.Name {
		t.Errorf("Search first record with inline struct")
	}

	TestDB.Where(&User{Name: user1.Name}).First(&user)
	if user.Id == 0 || user.Name != user1.Name {
		t.Errorf("Search first record with where struct")
	}

	TestDB.Find(&users, &User{Name: user2.Name})
	if len(users) != 1 {
		t.Errorf("Search all records with inline struct")
	}
}

func SearchWithMap(t *testing.T) {
	companyID := 1
	user1 := User{Name: "MapSearchUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "MapSearchUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "MapSearchUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	user4 := User{Name: "MapSearchUser4", Age: 30, Birthday: parseTime("2020-1-1"), CompanyID: &companyID}
	TestDB.Save(&user1).Save(&user2).Save(&user3).Save(&user4)

	var user User
	TestDB.First(&user, map[string]interface{}{"name": user1.Name})
	if user.Id == 0 || user.Name != user1.Name {
		t.Errorf("Search first record with inline map")
	}

	user = User{}
	TestDB.Where(map[string]interface{}{"name": user2.Name}).First(&user)
	if user.Id == 0 || user.Name != user2.Name {
		t.Errorf("Search first record with where map")
	}

	var users []User
	TestDB.Where(map[string]interface{}{"name": user3.Name}).Find(&users)
	if len(users) != 1 {
		t.Errorf("Search all records with inline map")
	}

	TestDB.Find(&users, map[string]interface{}{"name": user3.Name})
	if len(users) != 1 {
		t.Errorf("Search all records with inline map")
	}

	TestDB.Find(&users, map[string]interface{}{"name": user4.Name, "company_id": nil})
	if len(users) != 0 {
		t.Errorf("Search all records with inline map containing null value finding 0 records")
	}

	TestDB.Find(&users, map[string]interface{}{"name": user1.Name, "company_id": nil})
	if len(users) != 1 {
		t.Errorf("Search all records with inline map containing null value finding 1 record")
	}

	TestDB.Find(&users, map[string]interface{}{"name": user4.Name, "company_id": companyID})
	if len(users) != 1 {
		t.Errorf("Search all records with inline multiple value map")
	}
}

func SearchWithEmptyChain(t *testing.T) {
	user1 := User{Name: "ChainSearchUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "ChainearchUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "ChainearchUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	if TestDB.Where("").Where("").First(&User{}).Error != nil {
		t.Errorf("Should not raise any error if searching with empty strings")
	}

	if TestDB.Where(&User{}).Where("name = ?", user1.Name).First(&User{}).Error != nil {
		t.Errorf("Should not raise any error if searching with empty struct")
	}

	if TestDB.Where(map[string]interface{}{}).Where("name = ?", user1.Name).First(&User{}).Error != nil {
		t.Errorf("Should not raise any error if searching with empty map")
	}
}

func Select(t *testing.T) {
	user1 := User{Name: "SelectUser1"}
	TestDB.Save(&user1)

	var user User
	TestDB.Where("name = ?", user1.Name).Select("name").Find(&user)
	if user.Id != 0 {
		t.Errorf("Should not have ID because only selected name, %+v", user.Id)
	}

	if user.Name != user1.Name {
		t.Errorf("Should have user Name when selected it")
	}
}

func OrderAndPluck(t *testing.T) {
	user1 := User{Name: "OrderPluckUser1", Age: 1}
	user2 := User{Name: "OrderPluckUser2", Age: 10}
	user3 := User{Name: "OrderPluckUser3", Age: 20}
	TestDB.Save(&user1).Save(&user2).Save(&user3)
	scopedb := TestDB.Model(&User{}).Where("name like ?", "%OrderPluckUser%")

	var user User
	scopedb.Order(SqlExpr("name = ? DESC", "OrderPluckUser2")).First(&user)
	if user.Name != "OrderPluckUser2" {
		t.Errorf("Order with sql expression")
	}

	var ages []int64
	scopedb.Order("age desc").Pluck("age", &ages)
	if ages[0] != 20 {
		t.Errorf("The first age should be 20 when order with age desc")
	}

	var ages1, ages2 []int64
	scopedb.Order("age desc").Pluck("age", &ages1).Pluck("age", &ages2)
	if !reflect.DeepEqual(ages1, ages2) {
		t.Errorf("The first order is the primary order")
	}

	var ages3, ages4 []int64
	scopedb.Model(&User{}).Order("age desc").Pluck("age", &ages3).Order("age", true).Pluck("age", &ages4)
	if reflect.DeepEqual(ages3, ages4) {
		t.Errorf("Reorder should work")
	}

	var names []string
	var ages5 []int64
	scopedb.Model(User{}).Order("name").Order("age desc").Pluck("age", &ages5).Pluck("name", &names)
	if names != nil && ages5 != nil {
		if !(names[0] == user1.Name && names[1] == user2.Name && names[2] == user3.Name && ages5[2] == 20) {
			t.Errorf("Order with multiple orders")
		}
	} else {
		t.Errorf("Order with multiple orders")
	}

	TestDB.Model(User{}).Select("name, age").Find(&[]User{})
}

func Limit(t *testing.T) {
	user1 := User{Name: "LimitUser1", Age: 1}
	user2 := User{Name: "LimitUser2", Age: 10}
	user3 := User{Name: "LimitUser3", Age: 20}
	user4 := User{Name: "LimitUser4", Age: 10}
	user5 := User{Name: "LimitUser5", Age: 20}
	TestDB.Save(&user1).Save(&user2).Save(&user3).Save(&user4).Save(&user5)

	var users1, users2, users3 []User
	TestDB.Order("age desc").Limit(3).Find(&users1).Limit(5).Find(&users2).Limit(-1).Find(&users3)

	if len(users1) != 3 || len(users2) != 5 || len(users3) <= 5 {
		t.Errorf("Limit should works")
	}
}

func Offset(t *testing.T) {
	for i := 0; i < 20; i++ {
		TestDB.Save(&User{Name: fmt.Sprintf("OffsetUser%v", i)})
	}
	var users1, users2, users3, users4 []User
	TestDB.Limit(100).Order("age desc").Find(&users1).Offset(3).Find(&users2).Offset(5).Find(&users3).Offset(-1).Find(&users4)

	if (len(users1) != len(users4)) || (len(users1)-len(users2) != 3) || (len(users1)-len(users3) != 5) {
		t.Errorf("Offset should work")
	}
}

func Or(t *testing.T) {
	user1 := User{Name: "OrUser1", Age: 1}
	user2 := User{Name: "OrUser2", Age: 10}
	user3 := User{Name: "OrUser3", Age: 20}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	var users []User
	TestDB.Where("name = ?", user1.Name).Or("name = ?", user2.Name).Find(&users)
	if len(users) != 2 {
		t.Errorf("Find users with or")
	}
}

func Count(t *testing.T) {
	user1 := User{Name: "CountUser1", Age: 1}
	user2 := User{Name: "CountUser2", Age: 10}
	user3 := User{Name: "CountUser3", Age: 20}

	TestDB.Save(&user1).Save(&user2).Save(&user3)
	var count, count1, count2 int64
	var users []User

	if err := TestDB.Where("name = ?", user1.Name).Or("name = ?", user3.Name).Find(&users).Count(&count).Error; err != nil {
		t.Errorf(fmt.Sprintf("Count should work, but got err %v", err))
	}

	if count != int64(len(users)) {
		t.Errorf("Count() method should get correct value")
	}

	TestDB.Model(&User{}).Where("name = ?", user1.Name).Count(&count1).Or("name in (?)", []string{user2.Name, user3.Name}).Count(&count2)
	if count1 != 1 || count2 != 3 {
		t.Errorf("Multiple count in chain")
	}
}

func Not(t *testing.T) {
	TestDB.Unscoped().Delete(User{})
	TestDB.Create(getPreparedUser("user1", "not"))
	TestDB.Create(getPreparedUser("user2", "not"))
	TestDB.Create(getPreparedUser("user3", "not"))

	user4 := getPreparedUser("user4", "not")
	user4.Company = Company{}
	TestDB.Create(user4)

	var users1, users2, users3, users4, users5, users6, users7, users8, users9 []User
	rowsAffected := TestDB.Where("role = ?", "not").Find(&users1).RowsAffected
	if rowsAffected != 4 {
		t.Errorf("should find 4 `not` users")
	} else {
	}
	TestDB.Not(users1[0].Id).Find(&users2)

	if len(users1)-len(users2) != 1 {
		t.Errorf("Should ignore the first users with Not")
	} else {
	}

	TestDB.Not([]int{}).Find(&users3)
	if len(users1)-len(users3) != 0 {
		t.Errorf("Should find all users with a blank condition")
	} else {
	}

	var name3Count int64
	TestDB.Table("users").Where("name = ?", "user3").Count(&name3Count)
	TestDB.Not("name", "user3").Find(&users4)
	if len(users1)-len(users4) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	TestDB.Not("name = ?", "user3").Find(&users4)
	if len(users1)-len(users4) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	TestDB.Not("name <> ?", "user3").Find(&users4)
	if len(users4) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	TestDB.Not(User{Name: "user3"}).Find(&users5)

	if len(users1)-len(users5) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	TestDB.Not(map[string]interface{}{"name": "user3"}).Find(&users6)
	if len(users1)-len(users6) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	TestDB.Not(map[string]interface{}{"name": "user3", "company_id": nil}).Find(&users7)
	if len(users1)-len(users7) != 2 { // not user3 or user4
		t.Errorf("Should find all user's name not equal to 3 who do not have company id")
	} else {
	}

	TestDB.Not("name", []string{"user3"}).Find(&users8)
	if len(users1)-len(users8) != int(name3Count) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}

	var name2Count int64
	TestDB.Table("users").Where("name = ?", "user2").Count(&name2Count)
	TestDB.Not("name", []string{"user3", "user2"}).Find(&users9)
	if len(users1)-len(users9) != (int(name3Count) + int(name2Count)) {
		t.Errorf("Should find all users's name not equal 3")
	} else {
	}
}

func FillSmallerStruct(t *testing.T) {
	user1 := User{Name: "SmallerUser", Age: 100}
	TestDB.Save(&user1)
	type SimpleUser struct {
		Name      string
		Id        int64
		UpdatedAt time.Time
		CreatedAt time.Time
	}

	var simpleUser SimpleUser
	TestDB.Table("users").Where("name = ?", user1.Name).First(&simpleUser)

	if simpleUser.Id == 0 || simpleUser.Name == "" {
		t.Errorf("Should fill data correctly into smaller struct")
	}
}

func FindOrInitialize(t *testing.T) {
	var user1, user2, user3, user4, user5, user6 User
	TestDB.Where(&User{Name: "find or init", Age: 33}).FirstOrInit(&user1)
	if user1.Name != "find or init" || user1.Id != 0 || user1.Age != 33 {
		t.Errorf("user should be initialized with search value")
	}

	TestDB.Where(User{Name: "find or init", Age: 33}).FirstOrInit(&user2)
	if user2.Name != "find or init" || user2.Id != 0 || user2.Age != 33 {
		t.Errorf("user should be initialized with search value")
	}

	TestDB.FirstOrInit(&user3, map[string]interface{}{"name": "find or init 2"})
	if user3.Name != "find or init 2" || user3.Id != 0 {
		t.Errorf("user should be initialized with inline search value")
	}

	TestDB.Where(&User{Name: "find or init"}).Attrs(User{Age: 44}).FirstOrInit(&user4)
	if user4.Name != "find or init" || user4.Id != 0 || user4.Age != 44 {
		t.Errorf("user should be initialized with search value and attrs")
	}

	TestDB.Where(&User{Name: "find or init"}).Assign("age", 44).FirstOrInit(&user4)
	if user4.Name != "find or init" || user4.Id != 0 || user4.Age != 44 {
		t.Errorf("user should be initialized with search value and assign attrs")
	}

	TestDB.Save(&User{Name: "find or init", Age: 33})
	TestDB.Where(&User{Name: "find or init"}).Attrs("age", 44).FirstOrInit(&user5)
	if user5.Name != "find or init" || user5.Id == 0 || user5.Age != 33 {
		t.Errorf("user should be found and not initialized by Attrs")
	}

	TestDB.Where(&User{Name: "find or init", Age: 33}).FirstOrInit(&user6)
	if user6.Name != "find or init" || user6.Id == 0 || user6.Age != 33 {
		t.Errorf("user should be found with FirstOrInit")
	}

	TestDB.Where(&User{Name: "find or init"}).Assign(User{Age: 44}).FirstOrInit(&user6)
	if user6.Name != "find or init" || user6.Id == 0 || user6.Age != 44 {
		t.Errorf("user should be found and updated with assigned attrs")
	}
}

func FindOrCreate(t *testing.T) {
	var user1, user2, user3, user4, user5, user6, user7, user8 User
	TestDB.Where(&User{Name: "find or create", Age: 33}).FirstOrCreate(&user1)
	if user1.Name != "find or create" || user1.Id == 0 || user1.Age != 33 {
		t.Errorf("user should be created with search value")
	}

	TestDB.Where(&User{Name: "find or create", Age: 33}).FirstOrCreate(&user2)
	if user1.Id != user2.Id || user2.Name != "find or create" || user2.Id == 0 || user2.Age != 33 {
		t.Errorf("user should be created with search value")
	}

	TestDB.FirstOrCreate(&user3, map[string]interface{}{"name": "find or create 2"})
	if user3.Name != "find or create 2" || user3.Id == 0 {
		t.Errorf("user should be created with inline search value")
	}

	TestDB.Where(&User{Name: "find or create 3"}).Attrs("age", 44).FirstOrCreate(&user4)
	if user4.Name != "find or create 3" || user4.Id == 0 || user4.Age != 44 {
		t.Errorf("user should be created with search value and attrs")
	}

	updatedAt1 := user4.UpdatedAt
	TestDB.Where(&User{Name: "find or create 3"}).Assign("age", 55).FirstOrCreate(&user4)
	if updatedAt1.Format(time.RFC3339Nano) == user4.UpdatedAt.Format(time.RFC3339Nano) {
		t.Errorf("UpdateAt should be changed when update values with assign")
	}

	user4.Id = 0 //otherwise, will have duplicate entry error
	TestDB.Where(&User{Name: "find or create 4"}).Assign(User{Age: 44}).FirstOrCreate(&user4)
	if user4.Name != "find or create 4" || user4.Id == 0 || user4.Age != 44 {
		t.Errorf("user should be created with search value and assigned attrs")
	}

	TestDB.Where(&User{Name: "find or create"}).Attrs("age", 44).FirstOrInit(&user5)
	if user5.Name != "find or create" || user5.Id == 0 || user5.Age != 33 {
		t.Errorf("user should be found and not initialized by Attrs")
	}

	TestDB.Where(&User{Name: "find or create"}).Assign(User{Age: 44}).FirstOrCreate(&user6)
	if user6.Name != "find or create" || user6.Id == 0 || user6.Age != 44 {
		t.Errorf("user should be found and updated with assigned attrs")
	}

	TestDB.Where(&User{Name: "find or create"}).Find(&user7)
	if user7.Name != "find or create" || user7.Id == 0 || user7.Age != 44 {
		t.Errorf("user should be found and updated with assigned attrs")
	}

	TestDB.Where(&User{Name: "find or create embedded struct"}).Assign(User{Age: 44, CreditCard: CreditCard{Number: "1231231231"}, Emails: []Email{{Email: "jinzhu@assign_embedded_struct.com"}, {Email: "jinzhu-2@assign_embedded_struct.com"}}}).FirstOrCreate(&user8)
	if TestDB.Where("email = ?", "jinzhu-2@assign_embedded_struct.com").First(&Email{}).RecordNotFound() {
		t.Errorf("embedded struct email should be saved")
	}

	if TestDB.Where("number = ?", "1231231231").First(&CreditCard{}).RecordNotFound() {
		t.Errorf("embedded struct credit card should be saved")
	}
}

func SelectWithEscapedFieldName(t *testing.T) {
	user1 := User{Name: "EscapedFieldNameUser", Age: 1}
	user2 := User{Name: "EscapedFieldNameUser", Age: 10}
	user3 := User{Name: "EscapedFieldNameUser", Age: 20}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	var names []string
	TestDB.Model(User{}).Where(&User{Name: "EscapedFieldNameUser"}).Pluck("\"name\"", &names)

	if len(names) != 3 {
		t.Errorf("Expected 3 name, but got: %d", len(names))
	}
}

func SelectWithVariables(t *testing.T) {
	TestDB.Save(&User{Name: "jinzhu"})

	rows, _ := TestDB.Table("users").Select("? as fake", SqlExpr("name")).Rows()

	if !rows.Next() {
		t.Errorf("Should have returned at least one row")
	} else {
		columns, _ := rows.Columns()
		if !reflect.DeepEqual(columns, []string{"fake"}) {
			t.Errorf("Should only contains one column")
		}
	}

	rows.Close()
}
