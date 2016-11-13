package tests

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"gorm"
)

func TestSkipSaveAssociation(t *testing.T) {
	t.Log("FEATURE : TestSkipSaveAssociation")
	type Company struct {
		gorm.Model
		Name string
	}

	type User struct {
		gorm.Model
		Name      string
		CompanyID uint
		Company   Company `gorm:"save_associations:false"`
	}
	TestDB.AutoMigrate(&Company{}, &User{})

	TestDB.Save(&User{Name: "jinzhu", Company: Company{Name: "skip_save_association"}})

	if !TestDB.Where("name = ?", "skip_save_association").First(&Company{}).RecordNotFound() {
		t.Errorf("Company skip_save_association should not been saved")
	}
}

func TestBelongsTo(t *testing.T) {
	t.Log("25) TestBelongsTo")
	post := Post{
		Title:        "post belongs to",
		Body:         "body belongs to",
		Category:     Category{Name: "Category 1"},
		MainCategory: Category{Name: "Main Category 1"},
	}

	if err := TestDB.Save(&post).Error; err != nil {
		t.Error("Got errors when save post", err)
	}

	if post.Category.ID == 0 || post.MainCategory.ID == 0 {
		t.Errorf("Category's primary key should be updated")
	}

	if post.CategoryId.Int64 == 0 || post.MainCategoryId == 0 {
		t.Errorf("post's foreign key should be updated")
	}

	// Query
	var category1 Category
	TestDB.Model(&post).Association("Category").Find(&category1)
	if category1.Name != "Category 1" {
		t.Errorf("Query belongs to relations with Association")
	}

	var mainCategory1 Category
	TestDB.Model(&post).Association("MainCategory").Find(&mainCategory1)
	if mainCategory1.Name != "Main Category 1" {
		t.Errorf("Query belongs to relations with Association")
	}

	var category11 Category
	TestDB.Model(&post).Related(&category11)
	if category11.Name != "Category 1" {
		t.Errorf("Query belongs to relations with Related")
	}

	if TestDB.Model(&post).Association("Category").Count() != 1 {
		t.Errorf("Post's category count should be 1")
	}

	if TestDB.Model(&post).Association("MainCategory").Count() != 1 {
		t.Errorf("Post's main category count should be 1")
	}

	// Append
	var category2 = Category{
		Name: "Category 2",
	}
	TestDB.Model(&post).Association("Category").Append(&category2)

	if category2.ID == 0 {
		t.Errorf("Category should has ID when created with Append")
	}

	var category21 Category
	TestDB.Model(&post).Related(&category21)

	if category21.Name != "Category 2" {
		t.Errorf("Category should be updated with Append")
	}

	if TestDB.Model(&post).Association("Category").Count() != 1 {
		t.Errorf("Post's category count should be 1")
	}

	// Replace
	var category3 = Category{
		Name: "Category 3",
	}
	TestDB.Model(&post).Association("Category").Replace(&category3)

	if category3.ID == 0 {
		t.Errorf("Category should has ID when created with Replace")
	}

	var category31 Category
	TestDB.Model(&post).Related(&category31)
	if category31.Name != "Category 3" {
		t.Errorf("Category should be updated with Replace")
	}

	if TestDB.Model(&post).Association("Category").Count() != 1 {
		t.Errorf("Post's category count should be 1")
	}

	// Delete
	TestDB.Model(&post).Association("Category").Delete(&category2)
	if TestDB.Model(&post).Related(&Category{}).RecordNotFound() {
		t.Errorf("Should not delete any category when Delete a unrelated Category")
	}

	if post.Category.Name == "" {
		t.Errorf("Post's category should not be reseted when Delete a unrelated Category")
	}

	TestDB.Model(&post).Association("Category").Delete(&category3)

	if post.Category.Name != "" {
		t.Errorf("Post's category should be reseted after Delete")
	}

	var category41 Category
	TestDB.Model(&post).Related(&category41)
	if category41.Name != "" {
		t.Errorf("Category should be deleted with Delete")
	}

	if count := TestDB.Model(&post).Association("Category").Count(); count != 0 {
		t.Errorf("Post's category count should be 0 after Delete, but got %v", count)
	}

	// Clear
	TestDB.Model(&post).Association("Category").Append(&Category{
		Name: "Category 2",
	})

	if TestDB.Model(&post).Related(&Category{}).RecordNotFound() {
		t.Errorf("Should find category after append")
	}

	if post.Category.Name == "" {
		t.Errorf("Post's category should has value after Append")
	}

	TestDB.Model(&post).Association("Category").Clear()

	if post.Category.Name != "" {
		t.Errorf("Post's category should be cleared after Clear")
	}

	if !TestDB.Model(&post).Related(&Category{}).RecordNotFound() {
		t.Errorf("Should not find any category after Clear")
	}

	if count := TestDB.Model(&post).Association("Category").Count(); count != 0 {
		t.Errorf("Post's category count should be 0 after Clear, but got %v", count)
	}

	// Check Association mode with soft delete
	category6 := Category{
		Name: "Category 6",
	}
	TestDB.Model(&post).Association("Category").Append(&category6)

	if count := TestDB.Model(&post).Association("Category").Count(); count != 1 {
		t.Errorf("Post's category count should be 1 after Append, but got %v", count)
	}

	TestDB.Delete(&category6)

	if count := TestDB.Model(&post).Association("Category").Count(); count != 0 {
		t.Errorf("Post's category count should be 0 after the category has been deleted, but got %v", count)
	}

	if err := TestDB.Model(&post).Association("Category").Find(&Category{}).Error; err == nil {
		t.Errorf("Post's category is not findable after Delete")
	}

	if count := TestDB.Unscoped().Model(&post).Association("Category").Count(); count != 1 {
		t.Errorf("Post's category count should be 1 when query with Unscoped, but got %v", count)
	}

	if err := TestDB.Unscoped().Model(&post).Association("Category").Find(&Category{}).Error; err != nil {
		t.Errorf("Post's category should be findable when query with Unscoped, got %v", err)
	}
}

func TestBelongsToOverrideForeignKey1(t *testing.T) {
	t.Log("26) TestBelongsToOverrideForeignKey1")
	type Profile struct {
		gorm.Model
		Name string
	}

	type User struct {
		gorm.Model
		Profile      Profile `gorm:"ForeignKey:ProfileRefer"`
		ProfileRefer int
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.BELONGS_TO ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"ProfileRefer"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"ID"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestBelongsToOverrideForeignKey2(t *testing.T) {
	t.Log("27) TestBelongsToOverrideForeignKey2")
	type Profile struct {
		gorm.Model
		Refer string
		Name  string
	}

	type User struct {
		gorm.Model
		Profile   Profile `gorm:"ForeignKey:ProfileID;AssociationForeignKey:Refer"`
		ProfileID int
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.BELONGS_TO ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"ProfileID"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"Refer"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestHasOne(t *testing.T) {
	t.Log("28) TestHasOne")
	user := User{
		Name:       "has one",
		CreditCard: CreditCard{Number: "411111111111"},
	}

	if err := TestDB.Save(&user).Error; err != nil {
		t.Error("Got errors when save user", err.Error())
	}

	if user.CreditCard.UserId.Int64 == 0 {
		t.Errorf("CreditCard's foreign key should be updated")
	}

	// Query
	var creditCard1 CreditCard
	TestDB.Model(&user).Related(&creditCard1)

	if creditCard1.Number != "411111111111" {
		t.Errorf("Query has one relations with Related")
	}

	var creditCard11 CreditCard
	TestDB.Model(&user).Association("CreditCard").Find(&creditCard11)

	if creditCard11.Number != "411111111111" {
		t.Errorf("Query has one relations with Related")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 1 {
		t.Errorf("User's credit card count should be 1")
	}

	// Append
	var creditcard2 = CreditCard{
		Number: "411111111112",
	}
	TestDB.Model(&user).Association("CreditCard").Append(&creditcard2)

	if creditcard2.ID == 0 {
		t.Errorf("Creditcard should has ID when created with Append")
	}

	var creditcard21 CreditCard
	TestDB.Model(&user).Related(&creditcard21)
	if creditcard21.Number != "411111111112" {
		t.Errorf("CreditCard should be updated with Append")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 1 {
		t.Errorf("User's credit card count should be 1")
	}

	// Replace
	var creditcard3 = CreditCard{
		Number: "411111111113",
	}
	TestDB.Model(&user).Association("CreditCard").Replace(&creditcard3)

	if creditcard3.ID == 0 {
		t.Errorf("Creditcard should has ID when created with Replace")
	}

	var creditcard31 CreditCard
	TestDB.Model(&user).Related(&creditcard31)
	if creditcard31.Number != "411111111113" {
		t.Errorf("CreditCard should be updated with Replace")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 1 {
		t.Errorf("User's credit card count should be 1")
	}

	// Delete
	TestDB.Model(&user).Association("CreditCard").Delete(&creditcard2)
	var creditcard4 CreditCard
	TestDB.Model(&user).Related(&creditcard4)
	if creditcard4.Number != "411111111113" {
		t.Errorf("Should not delete credit card when Delete a unrelated CreditCard")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 1 {
		t.Errorf("User's credit card count should be 1")
	}

	TestDB.Model(&user).Association("CreditCard").Delete(&creditcard3)
	if !TestDB.Model(&user).Related(&CreditCard{}).RecordNotFound() {
		t.Errorf("Should delete credit card with Delete")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 0 {
		t.Errorf("User's credit card count should be 0 after Delete")
	}

	// Clear
	var creditcard5 = CreditCard{
		Number: "411111111115",
	}
	TestDB.Model(&user).Association("CreditCard").Append(&creditcard5)

	if TestDB.Model(&user).Related(&CreditCard{}).RecordNotFound() {
		t.Errorf("Should added credit card with Append")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 1 {
		t.Errorf("User's credit card count should be 1")
	}

	TestDB.Model(&user).Association("CreditCard").Clear()
	if !TestDB.Model(&user).Related(&CreditCard{}).RecordNotFound() {
		t.Errorf("Credit card should be deleted with Clear")
	}

	if TestDB.Model(&user).Association("CreditCard").Count() != 0 {
		t.Errorf("User's credit card count should be 0 after Clear")
	}

	// Check Association mode with soft delete
	var creditcard6 = CreditCard{
		Number: "411111111116",
	}
	TestDB.Model(&user).Association("CreditCard").Append(&creditcard6)

	if count := TestDB.Model(&user).Association("CreditCard").Count(); count != 1 {
		t.Errorf("User's credit card count should be 1 after Append, but got %v", count)
	}

	TestDB.Delete(&creditcard6)

	if count := TestDB.Model(&user).Association("CreditCard").Count(); count != 0 {
		t.Errorf("User's credit card count should be 0 after credit card deleted, but got %v", count)
	}

	if err := TestDB.Model(&user).Association("CreditCard").Find(&CreditCard{}).Error; err == nil {
		t.Errorf("User's creditcard is not findable after Delete")
	}

	if count := TestDB.Unscoped().Model(&user).Association("CreditCard").Count(); count != 1 {
		t.Errorf("User's credit card count should be 1 when query with Unscoped, but got %v", count)
	}

	if err := TestDB.Unscoped().Model(&user).Association("CreditCard").Find(&CreditCard{}).Error; err != nil {
		t.Errorf("User's creditcard should be findable when query with Unscoped, got %v", err)
	}
}

func TestHasOneOverrideForeignKey1(t *testing.T) {
	t.Log("29) TestHasOneOverrideForeignKey1")
	type Profile struct {
		gorm.Model
		Name      string
		UserRefer uint
	}

	type User struct {
		gorm.Model
		Profile Profile `gorm:"ForeignKey:UserRefer"`
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.HAS_ONE ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"UserRefer"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"ID"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestHasOneOverrideForeignKey2(t *testing.T) {
	t.Log("30) TestHasOneOverrideForeignKey2")
	type Profile struct {
		gorm.Model
		Name   string
		UserID uint
	}

	type User struct {
		gorm.Model
		Refer   string
		Profile Profile `gorm:"ForeignKey:UserID;AssociationForeignKey:Refer"`
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.HAS_ONE ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"UserID"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"Refer"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestHasMany(t *testing.T) {
	t.Log("31) TestHasMany")
	post := Post{
		Title:    "post has many",
		Body:     "body has many",
		Comments: []*Comment{{Content: "Comment 1"}, {Content: "Comment 2"}},
	}

	if err := TestDB.Save(&post).Error; err != nil {
		t.Error("Got errors when save post")
		t.Errorf("ERROR : %v", err)
	}

	for _, comment := range post.Comments {
		if comment.PostId == 0 {
			t.Errorf("comment's PostID should be updated")
		}
	}

	var compareComments = func(comments []Comment, contents []string) bool {
		var commentContents []string
		for _, comment := range comments {
			commentContents = append(commentContents, comment.Content)
		}
		sort.Strings(commentContents)
		sort.Strings(contents)
		return reflect.DeepEqual(commentContents, contents)
	}

	// Query
	if TestDB.First(&Comment{}, "content = ?", "Comment 1").Error != nil {
		t.Errorf("Comment 1 should be saved")
	}

	var comments1 []Comment
	TestDB.Model(&post).Association("Comments").Find(&comments1)
	if !compareComments(comments1, []string{"Comment 1", "Comment 2"}) {
		t.Errorf("Query has many relations with Association")
	}

	var comments11 []Comment
	TestDB.Model(&post).Related(&comments11)
	if !compareComments(comments11, []string{"Comment 1", "Comment 2"}) {
		t.Errorf("Query has many relations with Related")
	}

	if TestDB.Model(&post).Association("Comments").Count() != 2 {
		t.Errorf("Post's comments count should be 2")
	}

	// Append
	TestDB.Model(&post).Association("Comments").Append(&Comment{Content: "Comment 3"})

	var comments2 []Comment
	TestDB.Model(&post).Related(&comments2)
	if !compareComments(comments2, []string{"Comment 1", "Comment 2", "Comment 3"}) {
		t.Errorf("Append new record to has many relations")
	}

	if TestDB.Model(&post).Association("Comments").Count() != 3 {
		t.Errorf("Post's comments count should be 3 after Append")
	}

	// Delete
	TestDB.Model(&post).Association("Comments").Delete(comments11)

	var comments3 []Comment
	TestDB.Model(&post).Related(&comments3)
	if !compareComments(comments3, []string{"Comment 3"}) {
		t.Errorf("Delete an existing resource for has many relations")
	}

	if TestDB.Model(&post).Association("Comments").Count() != 1 {
		t.Errorf("Post's comments count should be 1 after Delete 2")
	}

	// Replace
	TestDB.Model(&Post{Id: 999}).Association("Comments").Replace()

	var comments4 []Comment
	TestDB.Model(&post).Related(&comments4)
	if len(comments4) == 0 {
		t.Errorf("Replace for other resource should not clear all comments")
	}

	TestDB.Model(&post).Association("Comments").Replace(&Comment{Content: "Comment 4"}, &Comment{Content: "Comment 5"})

	var comments41 []Comment
	TestDB.Model(&post).Related(&comments41)
	if !compareComments(comments41, []string{"Comment 4", "Comment 5"}) {
		t.Errorf("Replace has many relations")
	}

	// Clear
	TestDB.Model(&Post{Id: 999}).Association("Comments").Clear()

	var comments5 []Comment
	TestDB.Model(&post).Related(&comments5)
	if len(comments5) == 0 {
		t.Errorf("Clear should not clear all comments")
	}

	TestDB.Model(&post).Association("Comments").Clear()

	var comments51 []Comment
	TestDB.Model(&post).Related(&comments51)
	if len(comments51) != 0 {
		t.Errorf("Clear has many relations")
	}

	// Check Association mode with soft delete
	var comment6 = Comment{
		Content: "comment 6",
	}
	TestDB.Model(&post).Association("Comments").Append(&comment6)

	if count := TestDB.Model(&post).Association("Comments").Count(); count != 1 {
		t.Errorf("post's comments count should be 1 after Append, but got %v", count)
	}

	TestDB.Delete(&comment6)

	if count := TestDB.Model(&post).Association("Comments").Count(); count != 0 {
		t.Errorf("post's comments count should be 0 after comment been deleted, but got %v", count)
	}

	var comments6 []Comment
	if TestDB.Model(&post).Association("Comments").Find(&comments6); len(comments6) != 0 {
		t.Errorf("post's comments count should be 0 when find with Find, but got %v", len(comments6))
	}

	if count := TestDB.Unscoped().Model(&post).Association("Comments").Count(); count != 1 {
		t.Errorf("post's comments count should be 1 when query with Unscoped, but got %v", count)
	}

	var comments61 []Comment
	if TestDB.Unscoped().Model(&post).Association("Comments").Find(&comments61); len(comments61) != 1 {
		t.Errorf("post's comments count should be 1 when query with Unscoped, but got %v", len(comments61))
	}
}

func TestHasManyOverrideForeignKey1(t *testing.T) {
	t.Log("32) TestHasManyOverrideForeignKey1")
	type Profile struct {
		gorm.Model
		Name      string
		UserRefer uint
	}

	type User struct {
		gorm.Model
		Profile []Profile `gorm:"ForeignKey:UserRefer"`
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.HAS_MANY ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"UserRefer"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"ID"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestHasManyOverrideForeignKey2(t *testing.T) {
	t.Log("33) TestHasManyOverrideForeignKey2")
	type Profile struct {
		gorm.Model
		Name   string
		UserID uint
	}

	type User struct {
		gorm.Model
		Refer   string
		Profile []Profile `gorm:"ForeignKey:UserID;AssociationForeignKey:Refer"`
	}

	if relation, ok := TestDB.NewScope(&User{}).FieldByName("Profile"); ok {
		if relation.Relationship.Kind != gorm.HAS_MANY ||
			!reflect.DeepEqual(relation.Relationship.ForeignFieldNames, gorm.StrSlice{"UserID"}) ||
			!reflect.DeepEqual(relation.Relationship.AssociationForeignFieldNames, gorm.StrSlice{"Refer"}) {
			t.Errorf("Override belongs to foreign key with tag")
		}
	}
}

func TestManyToMany(t *testing.T) {
	t.Log("34) TestManyToMany")
	TestDB.Raw("delete from languages")
	var languages = []Language{{Name: "ZH"}, {Name: "EN"}}
	user := User{Name: "Many2Many", Languages: languages}
	TestDB.Save(&user)

	// Query
	var newLanguages []Language
	TestDB.Model(&user).Related(&newLanguages, "Languages")
	if len(newLanguages) != len([]string{"ZH", "EN"}) {
		t.Errorf("Query many to many relations")
	}

	TestDB.Model(&user).Association("Languages").Find(&newLanguages)
	if len(newLanguages) != len([]string{"ZH", "EN"}) {
		t.Errorf("Should be able to find many to many relations")
	}

	if TestDB.Model(&user).Association("Languages").Count() != len([]string{"ZH", "EN"}) {
		t.Errorf("Count should return correct result")
	}

	// Append
	TestDB.Model(&user).Association("Languages").Append(&Language{Name: "DE"})
	if TestDB.Where("name = ?", "DE").First(&Language{}).RecordNotFound() {
		t.Errorf("New record should be saved when append")
	}

	languageA := Language{Name: "AA"}
	TestDB.Save(&languageA)
	TestDB.Model(&User{Id: user.Id}).Association("Languages").Append(&languageA)

	languageC := Language{Name: "CC"}
	TestDB.Save(&languageC)
	TestDB.Model(&user).Association("Languages").Append(&[]Language{{Name: "BB"}, languageC})

	TestDB.Model(&User{Id: user.Id}).Association("Languages").Append(&[]Language{{Name: "DD"}, {Name: "EE"}})

	totalLanguages := []string{"ZH", "EN", "DE", "AA", "BB", "CC", "DD", "EE"}

	if TestDB.Model(&user).Association("Languages").Count() != len(totalLanguages) {
		t.Errorf("All appended languages should be saved")
	}

	// Delete
	user.Languages = []Language{}
	TestDB.Model(&user).Association("Languages").Find(&user.Languages)

	var language Language
	TestDB.Where("name = ?", "EE").First(&language)
	TestDB.Model(&user).Association("Languages").Delete(language, &language)

	if TestDB.Model(&user).Association("Languages").Count() != len(totalLanguages)-1 || len(user.Languages) != len(totalLanguages)-1 {
		t.Errorf("Relations should be deleted with Delete")
	}
	if TestDB.Where("name = ?", "EE").First(&Language{}).RecordNotFound() {
		t.Errorf("Language EE should not be deleted")
	}

	TestDB.Where("name IN (?)", []string{"CC", "DD"}).Find(&languages)

	user2 := User{Name: "Many2Many_User2", Languages: languages}
	TestDB.Save(&user2)

	TestDB.Model(&user).Association("Languages").Delete(languages, &languages)
	if TestDB.Model(&user).Association("Languages").Count() != len(totalLanguages)-3 || len(user.Languages) != len(totalLanguages)-3 {
		t.Errorf("Relations should be deleted with Delete")
	}

	if TestDB.Model(&user2).Association("Languages").Count() == 0 {
		t.Errorf("Other user's relations should not be deleted")
	}

	// Replace
	var languageB Language
	TestDB.Where("name = ?", "BB").First(&languageB)
	TestDB.Model(&user).Association("Languages").Replace(languageB)
	if len(user.Languages) != 1 || TestDB.Model(&user).Association("Languages").Count() != 1 {
		t.Errorf("Relations should be replaced")
	}

	TestDB.Model(&user).Association("Languages").Replace()
	if len(user.Languages) != 0 || TestDB.Model(&user).Association("Languages").Count() != 0 {
		t.Errorf("Relations should be replaced with empty")
	}

	TestDB.Model(&user).Association("Languages").Replace(&[]Language{{Name: "FF"}, {Name: "JJ"}})
	if len(user.Languages) != 2 || TestDB.Model(&user).Association("Languages").Count() != len([]string{"FF", "JJ"}) {
		t.Errorf("Relations should be replaced")
	}

	// Clear
	TestDB.Model(&user).Association("Languages").Clear()
	if len(user.Languages) != 0 || TestDB.Model(&user).Association("Languages").Count() != 0 {
		t.Errorf("Relations should be cleared")
	}

	// Check Association mode with soft delete
	var language6 = Language{
		Name: "language 6",
	}
	TestDB.Model(&user).Association("Languages").Append(&language6)

	if count := TestDB.Model(&user).Association("Languages").Count(); count != 1 {
		t.Errorf("user's languages count should be 1 after Append, but got %v", count)
	}

	TestDB.Delete(&language6)

	if count := TestDB.Model(&user).Association("Languages").Count(); count != 0 {
		t.Errorf("user's languages count should be 0 after language been deleted, but got %v", count)
	}

	var languages6 []Language
	if TestDB.Model(&user).Association("Languages").Find(&languages6); len(languages6) != 0 {
		t.Errorf("user's languages count should be 0 when find with Find, but got %v", len(languages6))
	}

	if count := TestDB.Unscoped().Model(&user).Association("Languages").Count(); count != 1 {
		t.Errorf("user's languages count should be 1 when query with Unscoped, but got %v", count)
	}

	var languages61 []Language
	if TestDB.Unscoped().Model(&user).Association("Languages").Find(&languages61); len(languages61) != 1 {
		t.Errorf("user's languages count should be 1 when query with Unscoped, but got %v", len(languages61))
	}
}

func TestRelated(t *testing.T) {
	t.Log("35) TestRelated")
	user := User{
		Name:            "jinzhu",
		BillingAddress:  Address{Address1: "Billing Address - Address 1"},
		ShippingAddress: Address{Address1: "Shipping Address - Address 1"},
		Emails:          []Email{{Email: "jinzhu@example.com"}, {Email: "jinzhu-2@example@example.com"}},
		CreditCard:      CreditCard{Number: "1234567890"},
		Company:         Company{Name: "company1"},
	}

	if err := TestDB.Save(&user).Error; err != nil {
		t.Errorf("No error should happen when saving user")
		t.Errorf("ERROR : %v", err)
	}

	if user.CreditCard.ID == 0 {
		t.Errorf("After user save, credit card should have id")
	}

	if user.BillingAddress.ID == 0 {
		t.Errorf("After user save, billing address should have id")
	}

	if user.Emails[0].Id == 0 {
		t.Errorf("After user save, billing address should have id")
	}

	var emails []Email
	TestDB.Model(&user).Related(&emails)
	if len(emails) != 2 {
		t.Errorf("Should have two emails")
	}

	var emails2 []Email
	TestDB.Model(&user).Where("email = ?", "jinzhu@example.com").Related(&emails2)
	if len(emails2) != 1 {
		t.Errorf("Should have two emails")
	}

	var emails3 []*Email
	TestDB.Model(&user).Related(&emails3)
	if len(emails3) != 2 {
		t.Errorf("Should have two emails")
	}

	var user1 User
	TestDB.Model(&user).Related(&user1.Emails)
	if len(user1.Emails) != 2 {
		t.Errorf("Should have only one email match related condition")
	}

	var address1 Address
	TestDB.Model(&user).Related(&address1, "BillingAddressId")
	if address1.Address1 != "Billing Address - Address 1" {
		t.Errorf("Should get billing address from user correctly")
	}

	user1 = User{}
	TestDB.Model(&address1).Related(&user1, "BillingAddressId")
	if TestDB.NewRecord(user1) {
		t.Errorf("Should get user from address correctly")
	}

	var user2 User
	TestDB.Model(&emails[0]).Related(&user2)
	if user2.Id != user.Id || user2.Name != user.Name {
		t.Errorf("Should get user from email correctly")
	}

	var creditcard CreditCard
	var user3 User
	TestDB.First(&creditcard, "number = ?", "1234567890")
	TestDB.Model(&creditcard).Related(&user3)
	if user3.Id != user.Id || user3.Name != user.Name {
		t.Errorf("Should get user from credit card correctly")
	}

	if !TestDB.Model(&CreditCard{}).Related(&User{}).RecordNotFound() {
		t.Errorf("RecordNotFound for Related")
	}

	var company Company
	if TestDB.Model(&user).Related(&company, "Company").RecordNotFound() || company.Name != "company1" {
		t.Errorf("RecordNotFound for Related")
	}
}

func TestForeignKey(t *testing.T) {
	t.Log("36) TestForeignKey")
	for _, structField := range TestDB.NewScope(&User{}).GetModelStruct().StructFields() {
		for _, foreignKey := range []string{"BillingAddressID", "ShippingAddressId", "CompanyID"} {
			if structField.GetName() == foreignKey && !structField.IsForeignKey() {
				t.Errorf(fmt.Sprintf("%v should be foreign key", foreignKey))
			}
		}
	}

	for _, structField := range TestDB.NewScope(&Email{}).GetModelStruct().StructFields() {
		for _, foreignKey := range []string{"UserId"} {
			if structField.GetName() == foreignKey && !structField.IsForeignKey() {
				t.Errorf(fmt.Sprintf("%v should be foreign key", foreignKey))
			}
		}
	}

	for _, structField := range TestDB.NewScope(&Post{}).GetModelStruct().StructFields() {
		for _, foreignKey := range []string{"CategoryId", "MainCategoryId"} {
			if structField.GetName() == foreignKey && !structField.IsForeignKey() {
				t.Errorf(fmt.Sprintf("%v should be foreign key", foreignKey))
			}
		}
	}

	for _, structField := range TestDB.NewScope(&Comment{}).GetModelStruct().StructFields() {
		for _, foreignKey := range []string{"PostId"} {
			if structField.GetName() == foreignKey && !structField.IsForeignKey() {
				t.Errorf(fmt.Sprintf("%v should be foreign key", foreignKey))
			}
		}
	}
}

func testForeignKey(t *testing.T, source interface{}, sourceFieldName string, target interface{}, targetFieldName string) {
	if dialect := os.Getenv("GORM_DIALECT"); dialect == "" || dialect == "sqlite" {
		// sqlite does not support ADD CONSTRAINT in ALTER TABLE
		return
	}
	targetScope := TestDB.NewScope(target)
	targetTableName := targetScope.TableName()
	modelScope := TestDB.NewScope(source)
	modelField, ok := modelScope.FieldByName(sourceFieldName)
	if !ok {
		t.Fatalf(fmt.Sprintf("Failed to get field by name: %v", sourceFieldName))
	}
	targetField, ok := targetScope.FieldByName(targetFieldName)
	if !ok {
		t.Fatalf(fmt.Sprintf("Failed to get field by name: %v", targetFieldName))
	}
	dest := fmt.Sprintf("%v(%v)", targetTableName, targetField.DBName)
	err := TestDB.Model(source).AddForeignKey(modelField.DBName, dest, "CASCADE", "CASCADE").Error
	if err != nil {
		t.Fatalf(fmt.Sprintf("Failed to create foreign key: %v", err))
	}
}

func TestLongForeignKey(t *testing.T) {
	t.Log("37) TestLongForeignKey")
	testForeignKey(t, &NotSoLongTableName{}, "ReallyLongThingID", &ReallyLongTableNameToTestMySQLNameLengthLimit{}, "ID")
}

func TestLongForeignKeyWithShortDest(t *testing.T) {
	t.Log("38) TestLongForeignKeyWithShortDest")
	testForeignKey(t, &ReallyLongThingThatReferencesShort{}, "ShortID", &Short{}, "ID")
}

func TestHasManyChildrenWithOneStruct(t *testing.T) {
	t.Log("39) TestHasManyChildrenWithOneStruct")
	category := Category{
		Name: "main",
		Categories: []Category{
			{Name: "sub1"},
			{Name: "sub2"},
		},
	}

	TestDB.Save(&category)
}
