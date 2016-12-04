package tests

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"gorm"
	_ "gorm/dialects/mysql"
	_ "gorm/dialects/sqlite"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var (
	TestDB      *gorm.DBCon
	TestDBErr   error
	compareToys = func(toys []Toy, contents []string) bool {
		var toyContents []string
		for _, toy := range toys {
			toyContents = append(toyContents, toy.Name)
		}
		sort.Strings(toyContents)
		sort.Strings(contents)
		return reflect.DeepEqual(toyContents, contents)
	}

	memStats     runtime.MemStats
	measuresData []*Measure = make([]*Measure, 0, 0)
)

type (
	ElementWithIgnoredField struct {
		Id           int64
		Value        string
		IgnoredField int64 `sql:"-"`
	}

	RecordWithSlice struct {
		ID      uint64
		Strings ExampleStringSlice `sql:"type:text"`
		Structs ExampleStructSlice `sql:"type:text"`
	}

	ExampleStringSlice []string

	ExampleStruct struct {
		Name  string
		Value string
	}

	ExampleStructSlice []ExampleStruct

	BasePost struct {
		Id    int64
		Title string
		URL   string
	}

	Author struct {
		Name  string
		Email string
	}

	HNPost struct {
		BasePost
		Author  `gorm:"embedded_prefix:user_"` // Embedded struct
		Upvotes int32
	}

	EngadgetPost struct {
		BasePost BasePost `gorm:"embedded"`
		Author   Author   `gorm:"embedded;embedded_prefix:author_"` // Embedded struct
		ImageUrl string
	}

	LevelA1 struct {
		ID    uint
		Value string
	}

	LevelA2 struct {
		ID       uint
		Value    string
		LevelA3s []*LevelA3
	}

	LevelA3 struct {
		ID        uint
		Value     string
		LevelA1ID sql.NullInt64
		LevelA1   *LevelA1
		LevelA2ID sql.NullInt64
		LevelA2   *LevelA2
	}

	LevelB1 struct {
		ID       uint
		Value    string
		LevelB3s []*LevelB3
	}

	LevelB2 struct {
		ID    uint
		Value string
	}

	LevelB3 struct {
		ID        uint
		Value     string
		LevelB1ID sql.NullInt64
		LevelB1   *LevelB1
		LevelB2s  []*LevelB2 `gorm:"many2many:levelb1_levelb3_levelb2s"`
	}

	LevelC1 struct {
		ID        uint
		Value     string
		LevelC2ID uint
	}

	LevelC2 struct {
		ID      uint
		Value   string
		LevelC1 LevelC1
	}

	LevelC3 struct {
		ID        uint
		Value     string
		LevelC2ID uint
		LevelC2   LevelC2
	}

	Cat struct {
		Id   int
		Name string
		Toy  Toy `gorm:"polymorphic:Owner;"`
	}

	Dog struct {
		Id   int
		Name string
		Toys []Toy `gorm:"polymorphic:Owner;"`
	}

	Hamster struct {
		Id           int
		Name         string
		PreferredToy Toy `gorm:"polymorphic:Owner;polymorphic_value:hamster_preferred"`
		OtherToy     Toy `gorm:"polymorphic:Owner;polymorphic_value:hamster_other"`
	}

	Toy struct {
		Id        int
		Name      string
		OwnerId   int
		OwnerType string
	}

	PointerStruct struct {
		ID   int64
		Name *string
		Num  *int
	}

	NormalStruct struct {
		ID   int64
		Name string
		Num  int
	}

	NotSoLongTableName struct {
		Id                int64
		ReallyLongThingID int64
		ReallyLongThing   ReallyLongTableNameToTestMySQLNameLengthLimit
	}

	ReallyLongTableNameToTestMySQLNameLengthLimit struct {
		Id int64
	}

	ReallyLongThingThatReferencesShort struct {
		Id      int64
		ShortID int64
		Short   Short
	}

	Short struct {
		Id int64
	}

	Num int64

	User struct {
		Id                int64
		Age               int64
		UserNum           Num
		Name              string `sql:"size:255"`
		Email             string
		Birthday          *time.Time    // Time
		CreatedAt         time.Time     // CreatedAt: Time of record is created, will be insert automatically
		UpdatedAt         time.Time     // UpdatedAt: Time of record is updated, will be updated automatically
		Emails            []Email       // Embedded structs
		BillingAddress    Address       // Embedded struct
		BillingAddressID  sql.NullInt64 // Embedded struct's foreign key
		ShippingAddress   Address       // Embedded struct
		ShippingAddressId int64         // Embedded struct's foreign key
		CreditCard        CreditCard
		Latitude          float64
		Languages         []Language `gorm:"many2many:user_languages;"`
		CompanyID         *int
		Company           Company
		Role
		PasswordHash      []byte
		Sequence          uint                  `gorm:"AUTO_INCREMENT"`
		IgnoreMe          int64                 `sql:"-"`
		IgnoreStringSlice []string              `sql:"-"`
		Ignored           struct{ Name string } `sql:"-"`
		IgnoredPointer    *User                 `sql:"-"`
	}

	CreditCard struct {
		ID        int8
		Number    string
		UserId    sql.NullInt64
		CreatedAt time.Time `sql:"not null"`
		UpdatedAt time.Time
		DeletedAt *time.Time
	}

	Blog struct {
		ID         uint   `gorm:"primary_key"`
		Locale     string `gorm:"primary_key"`
		Subject    string
		Body       string
		Tags       []Tag `gorm:"many2many:blog_tags;"`
		SharedTags []Tag `gorm:"many2many:shared_blog_tags;ForeignKey:id;AssociationForeignKey:id"`
		LocaleTags []Tag `gorm:"many2many:locale_blog_tags;ForeignKey:id,locale;AssociationForeignKey:id"`
	}

	Tag struct {
		ID     uint   `gorm:"primary_key"`
		Locale string `gorm:"primary_key"`
		Value  string
		Blogs  []*Blog `gorm:"many2many:blogs_tags"`
	}

	Email struct {
		Id        int16
		UserId    int
		Email     string `sql:"type:varchar(100);"`
		CreatedAt time.Time
		UpdatedAt time.Time
	}

	Address struct {
		ID        int
		Address1  string
		Address2  string
		Post      string
		CreatedAt time.Time
		UpdatedAt time.Time
		DeletedAt *time.Time
	}

	Language struct {
		gorm.Model
		Name  string
		Users []User `gorm:"many2many:user_languages;"`
	}

	Product struct {
		Id                    int64
		Code                  string
		Price                 int64
		CreatedAt             time.Time
		UpdatedAt             time.Time
		AfterFindCallTimes    int64
		BeforeCreateCallTimes int64
		AfterCreateCallTimes  int64
		BeforeUpdateCallTimes int64
		AfterUpdateCallTimes  int64
		BeforeSaveCallTimes   int64
		AfterSaveCallTimes    int64
		BeforeDeleteCallTimes int64
		AfterDeleteCallTimes  int64
	}

	Company struct {
		Id    int64
		Name  string
		Owner *User `sql:"-"`
	}

	Role struct {
		Name string `gorm:"size:256"`
	}

	Animal struct {
		Counter    uint64    `gorm:"primary_key:yes"`
		Name       string    `sql:"DEFAULT:'galeone'"`
		From       string    //test reserved sql keyword as field name
		Age        time.Time `sql:"DEFAULT:current_timestamp"`
		unexported string    // unexported value
		CreatedAt  time.Time
		UpdatedAt  time.Time
	}

	JoinTable struct {
		From uint64
		To   uint64
		Time time.Time `sql:"default: null"`
	}

	Post struct {
		Id             int64
		CategoryId     sql.NullInt64
		MainCategoryId int64
		Title          string
		Body           string
		Comments       []*Comment
		Category       Category
		MainCategory   Category
	}

	Category struct {
		gorm.Model
		Name string

		Categories []Category
		CategoryID *uint
	}

	Comment struct {
		gorm.Model
		PostId  int64
		Content string
		Post    Post
	}

	// Scanner
	NullValue struct {
		Id      int64
		Name    sql.NullString  `sql:"not null"`
		Gender  *sql.NullString `sql:"not null"`
		Age     sql.NullInt64
		Male    sql.NullBool
		Height  sql.NullFloat64
		AddedAt NullTime
	}

	NullTime struct {
		Time  time.Time
		Valid bool
	}

	BigEmail struct {
		Id           int64
		UserId       int64
		Email        string     `sql:"index:idx_email_agent"`
		UserAgent    string     `sql:"index:idx_email_agent"`
		RegisteredAt *time.Time `sql:"unique_index"`
		CreatedAt    time.Time
		UpdatedAt    time.Time
	}

	MultipleIndexes struct {
		ID     int64
		UserID int64  `sql:"unique_index:uix_multipleindexes_user_name,uix_multipleindexes_user_email;index:idx_multipleindexes_user_other"`
		Name   string `sql:"unique_index:uix_multipleindexes_user_name"`
		Email  string `sql:"unique_index:,uix_multipleindexes_user_email"`
		Other  string `sql:"index:,idx_multipleindexes_user_other"`
	}

	Person struct {
		Id        int
		Name      string
		Addresses []*Address `gorm:"many2many:person_addresses;"`
	}

	PersonAddress struct {
		gorm.JoinTableHandler
		PersonID  int
		AddressID int
		DeletedAt *time.Time
		CreatedAt time.Time
	}

	CalculateField struct {
		gorm.Model
		Name     string
		Children []CalculateFieldChild
		Category CalculateFieldCategory
		EmbeddedField
	}

	EmbeddedField struct {
		EmbeddedName string `sql:"NOT NULL;DEFAULT:'hello'"`
	}

	CalculateFieldChild struct {
		gorm.Model
		CalculateFieldID uint
		Name             string
	}

	CalculateFieldCategory struct {
		gorm.Model
		CalculateFieldID uint
		Name             string
	}

	CustomizeColumn struct {
		ID   int64      `gorm:"column:mapped_id; primary_key:yes"`
		Name string     `gorm:"column:mapped_name"`
		Date *time.Time `gorm:"column:mapped_time"`
	}

	// Make sure an ignored field does not interfere with another field's custom
	// column name that matches the ignored field.
	CustomColumnAndIgnoredFieldClash struct {
		Body    string `sql:"-"`
		RawBody string `gorm:"column:body"`
	}

	CustomizePerson struct {
		IdPerson string             `gorm:"column:idPerson;primary_key:true"`
		Accounts []CustomizeAccount `gorm:"many2many:PersonAccount;associationforeignkey:idAccount;foreignkey:idPerson"`
	}

	CustomizeAccount struct {
		IdAccount string `gorm:"column:idAccount;primary_key:true"`
		Name      string
	}

	CustomizeUser struct {
		gorm.Model
		Email string `sql:"column:email_address"`
	}

	CustomizeInvitation struct {
		gorm.Model
		Address string         `sql:"column:invitation"`
		Person  *CustomizeUser `gorm:"foreignkey:Email;associationforeignkey:invitation"`
	}

	PromotionDiscount struct {
		gorm.Model
		Name     string
		Coupons  []*PromotionCoupon `gorm:"ForeignKey:discount_id"`
		Rule     *PromotionRule     `gorm:"ForeignKey:discount_id"`
		Benefits []PromotionBenefit `gorm:"ForeignKey:promotion_id"`
	}

	PromotionBenefit struct {
		gorm.Model
		Name        string
		PromotionID uint
		Discount    PromotionDiscount `gorm:"ForeignKey:promotion_id"`
	}

	PromotionCoupon struct {
		gorm.Model
		Code       string
		DiscountID uint
		Discount   PromotionDiscount
	}

	PromotionRule struct {
		gorm.Model
		Name       string
		Begin      *time.Time
		End        *time.Time
		DiscountID uint
		Discount   *PromotionDiscount
	}

	Order struct {
	}

	Cart struct {
	}

	Measure struct {
		name        string
		duration    time.Duration
		startAllocs uint64 // The initial states of memStats.Mallocs and memStats.TotalAlloc.
		startBytes  uint64
		netAllocs   uint64 // The net total of this test after being run.
		netBytes    uint64
		start       time.Time
	}
)

func getPreloadUser(name string) *User {
	return getPreparedUser(name, "Preload")
}

func checkUserHasPreloadData(user User, t *testing.T) {
	u := getPreloadUser(user.Name)
	if user.BillingAddress.Address1 != u.BillingAddress.Address1 {
		t.Error("Failed to preload user's BillingAddress")
	}

	if user.ShippingAddress.Address1 != u.ShippingAddress.Address1 {
		t.Error("Failed to preload user's ShippingAddress")
	}

	if user.CreditCard.Number != u.CreditCard.Number {
		t.Error("Failed to preload user's CreditCard")
	}

	if user.Company.Name != u.Company.Name {
		t.Error("Failed to preload user's Company")
	}

	if len(user.Emails) != len(u.Emails) {
		t.Error("Failed to preload user's Emails")
	} else {
		var found int
		for _, e1 := range u.Emails {
			for _, e2 := range user.Emails {
				if e1.Email == e2.Email {
					found++
					break
				}
			}
		}
		if found != len(u.Emails) {
			t.Error("Failed to preload user's email details")
		}
	}
}

func compareTags(tags []Tag, contents []string) bool {
	var tagContents []string
	for _, tag := range tags {
		tagContents = append(tagContents, tag.Value)
	}
	sort.Strings(tagContents)
	sort.Strings(contents)
	return reflect.DeepEqual(tagContents, contents)
}

func getPreparedUser(name string, role string) *User {
	var company Company
	TestDB.Where(Company{Name: role}).FirstOrCreate(&company)

	return &User{
		Name:            name,
		Age:             20,
		Role:            Role{role},
		BillingAddress:  Address{Address1: fmt.Sprintf("Billing Address %v", name)},
		ShippingAddress: Address{Address1: fmt.Sprintf("Shipping Address %v", name)},
		CreditCard:      CreditCard{Number: fmt.Sprintf("123456%v", name)},
		Emails: []Email{
			{Email: fmt.Sprintf("user_%v@example1.com", name)}, {Email: fmt.Sprintf("user_%v@example2.com", name)},
		},
		Company: company,
		Languages: []Language{
			{Name: fmt.Sprintf("lang_1_%v", name)},
			{Name: fmt.Sprintf("lang_2_%v", name)},
		},
	}
}

func equalFuncs(funcs gorm.ScopedFuncs, fnames []string) bool {
	var names []string
	for _, f := range funcs {
		fnames := strings.Split(runtime.FuncForPC(reflect.ValueOf(*f).Pointer()).Name(), ".")
		names = append(names, fnames[len(fnames)-1])
	}
	return reflect.DeepEqual(names, fnames)
}

func NameIn1And2(d *gorm.DBCon) *gorm.DBCon {
	return d.Where("name in (?)", []string{"ScopeUser1", "ScopeUser2"})
}

func NameIn2And3(d *gorm.DBCon) *gorm.DBCon {
	return d.Where("name in (?)", []string{"ScopeUser2", "ScopeUser3"})
}

func NameIn(names []string) func(d *gorm.DBCon) *gorm.DBCon {
	return func(d *gorm.DBCon) *gorm.DBCon {
		return d.Where("name in (?)", names)
	}
}

func create(s *gorm.Scope)        {}
func beforeCreate1(s *gorm.Scope) {}
func beforeCreate2(s *gorm.Scope) {}
func afterCreate1(s *gorm.Scope)  {}
func afterCreate2(s *gorm.Scope)  {}
func replaceCreate(s *gorm.Scope) {}

func (e ElementWithIgnoredField) TableName() string {
	return "element_with_ignored_field"
}

func (s *Product) BeforeCreate() (err error) {
	if s.Code == "Invalid" {
		err = errors.New("BeforeCreate invalid product")
	}
	s.BeforeCreateCallTimes = s.BeforeCreateCallTimes + 1
	return
}

func (s *Product) BeforeUpdate() (err error) {
	if s.Code == "dont_update" {
		err = errors.New("BeforeUpdate can't update")
	}
	s.BeforeUpdateCallTimes = s.BeforeUpdateCallTimes + 1
	return
}

func (s *Product) BeforeSave() (err error) {
	if s.Code == "dont_save" {
		err = errors.New("BeforeSave can't save")
	}
	s.BeforeSaveCallTimes = s.BeforeSaveCallTimes + 1
	return
}

func (s *Product) AfterFind() {
	s.AfterFindCallTimes = s.AfterFindCallTimes + 1
}

func (s *Product) AfterCreate(tx *gorm.DBCon) {
	tx.Model(s).UpdateColumn(Product{AfterCreateCallTimes: s.AfterCreateCallTimes + 1})
}

func (s *Product) AfterUpdate() {
	s.AfterUpdateCallTimes = s.AfterUpdateCallTimes + 1
}

func (s *Product) AfterSave() (err error) {
	if s.Code == "after_save_error" {
		err = errors.New("AfterSave can't save")
	}
	s.AfterSaveCallTimes = s.AfterSaveCallTimes + 1
	return
}

func (s *Product) BeforeDelete() (err error) {
	if s.Code == "dont_delete" {
		err = errors.New("BeforeDelete can't delete")
	}
	s.BeforeDeleteCallTimes = s.BeforeDeleteCallTimes + 1
	return
}

func (s *Product) AfterDelete() (err error) {
	if s.Code == "after_delete_error" {
		err = errors.New("AfterDelete can't delete")
	}
	s.AfterDeleteCallTimes = s.AfterDeleteCallTimes + 1
	return
}

func (s *Product) GetCallTimes() []int64 {
	return []int64{s.BeforeCreateCallTimes, s.BeforeSaveCallTimes, s.BeforeUpdateCallTimes, s.AfterCreateCallTimes, s.AfterSaveCallTimes, s.AfterUpdateCallTimes, s.BeforeDeleteCallTimes, s.AfterDeleteCallTimes, s.AfterFindCallTimes}
}

func (l ExampleStringSlice) Value() (driver.Value, error) {
	return json.Marshal(l)
}

func (l *ExampleStringSlice) Scan(input interface{}) error {
	switch value := input.(type) {
	case string:
		return json.Unmarshal([]byte(value), l)
	case []byte:
		return json.Unmarshal(value, l)
	default:
		return errors.New("not supported")
	}
}

func (l ExampleStructSlice) Value() (driver.Value, error) {
	return json.Marshal(l)
}

func (l *ExampleStructSlice) Scan(input interface{}) error {
	switch value := input.(type) {
	case string:
		return json.Unmarshal([]byte(value), l)
	case []byte:
		return json.Unmarshal(value, l)
	default:
		return errors.New("not supported")
	}
}

func (b BigEmail) TableName() string {
	return "emails"
}

func (c Cart) TableName() string {
	return "shopping_cart"
}

func (p Person) String() string {
	optionals := fmt.Sprintf("%q:%d,%q:%q",
		"id", p.Id,
		"type", p.Name)
	if len(p.Addresses) > 0 {
		optionals += fmt.Sprintf(",%q:%d", "addresses", len(p.Addresses))
	}
	return fmt.Sprint(optionals)
}

func (*PersonAddress) Add(handler gorm.JoinTableHandlerInterface, db *gorm.DBCon, foreignValue interface{}, associationValue interface{}) error {
	return db.Where(map[string]interface{}{
		"person_id":  db.NewScope(foreignValue).PrimaryKeyValue(),
		"address_id": db.NewScope(associationValue).PrimaryKeyValue(),
	}).Assign(map[string]interface{}{
		"person_id":  foreignValue,
		"address_id": associationValue,
		"deleted_at": gorm.SqlExpr("NULL"),
	}).FirstOrCreate(&PersonAddress{}).Error
}

func (*PersonAddress) Delete(handler gorm.JoinTableHandlerInterface, db *gorm.DBCon) error {
	return db.Delete(&PersonAddress{}).Error
}

func (pa *PersonAddress) JoinWith(handler gorm.JoinTableHandlerInterface, db *gorm.DBCon, source interface{}) *gorm.DBCon {
	table := pa.Table(db)
	return db.Joins("INNER JOIN person_addresses ON person_addresses.address_id = addresses.id").Where(fmt.Sprintf("%v.deleted_at IS NULL OR %v.deleted_at <= '0001-01-02'", table, table))
}

func (role *Role) Scan(value interface{}) error {
	if b, ok := value.([]uint8); ok {
		role.Name = string(b)
	} else {
		role.Name = value.(string)
	}
	return nil
}

func (role Role) Value() (driver.Value, error) {
	return role.Name, nil
}

func (role Role) IsAdmin() bool {
	return role.Name == "admin"
}

func (i *Num) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
	case int64:
		//TODO : @Badu - assignment to method receiver propagates only to callees but not to callers
		*i = Num(s)
	default:
		return errors.New("Cannot scan NamedInt from " + reflect.ValueOf(src).String())
	}
	return nil
}

func (nt *NullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Valid = false
		return nil
	}
	nt.Time, nt.Valid = value.(time.Time), true
	return nil
}

func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}
