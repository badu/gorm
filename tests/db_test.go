package tests

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/erikstmartin/go-testdb"

	"encoding/json"
	"github.com/jinzhu/now"
	_ "gorm/dialects/mysql"
	pgdialect "gorm/dialects/postgres"
	_ "gorm/dialects/sqlite"
	"sort"
	"strings"
	"gorm"
)

var (
	TestDB *gorm.DBCon

	compareToys = func(toys []Toy, contents []string) bool {
		var toyContents []string
		for _, toy := range toys {
			toyContents = append(toyContents, toy.Name)
		}
		sort.Strings(toyContents)
		sort.Strings(contents)
		return reflect.DeepEqual(toyContents, contents)
	}
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

func runMigration() {
	if err := TestDB.DropTableIfExists(&User{}).Error; err != nil {
		fmt.Printf("Got error when try to delete table users, %+v\n", err)
	}

	for _, table := range []string{"animals", "user_languages"} {
		TestDB.Exec(fmt.Sprintf("drop table %v;", table))
	}

	values := []interface{}{&Short{}, &ReallyLongThingThatReferencesShort{}, &ReallyLongTableNameToTestMySQLNameLengthLimit{}, &NotSoLongTableName{}, &Product{}, &Email{}, &Address{}, &CreditCard{}, &Company{}, &Role{}, &Language{}, &HNPost{}, &EngadgetPost{}, &Animal{}, &User{}, &JoinTable{}, &Post{}, &Category{}, &Comment{}, &Cat{}, &Dog{}, &Hamster{}, &Toy{}, &ElementWithIgnoredField{}}
	for _, value := range values {
		TestDB.DropTable(value)
	}
	if err := TestDB.AutoMigrate(values...).Error; err != nil {
		panic(fmt.Sprintf("No error should happen when create table, but got %+v", err))
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
		err = errors.New("invalid product")
	}
	s.BeforeCreateCallTimes = s.BeforeCreateCallTimes + 1
	return
}

func (s *Product) BeforeUpdate() (err error) {
	if s.Code == "dont_update" {
		err = errors.New("can't update")
	}
	s.BeforeUpdateCallTimes = s.BeforeUpdateCallTimes + 1
	return
}

func (s *Product) BeforeSave() (err error) {
	if s.Code == "dont_save" {
		err = errors.New("can't save")
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
		err = errors.New("can't save")
	}
	s.AfterSaveCallTimes = s.AfterSaveCallTimes + 1
	return
}

func (s *Product) BeforeDelete() (err error) {
	if s.Code == "dont_delete" {
		err = errors.New("can't delete")
	}
	s.BeforeDeleteCallTimes = s.BeforeDeleteCallTimes + 1
	return
}

func (s *Product) AfterDelete() (err error) {
	if s.Code == "after_delete_error" {
		err = errors.New("can't delete")
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
		"deleted_at": gorm.Expr("NULL"),
	}).FirstOrCreate(&PersonAddress{}).Error
}

func (*PersonAddress) Delete(handler gorm.JoinTableHandlerInterface, db *gorm.DBCon, sources ...interface{}) error {
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

func init() {
	var err error

	if TestDB, err = OpenTestConnection(); err != nil {
		panic(fmt.Sprintf("No error should happen when connecting to test database, but got err=%+v", err))
	}

	runMigration()
}

func toJSONString(v interface{}) []byte {
	r, _ := json.MarshalIndent(v, "", "  ")
	return r
}

func parseTime(str string) *time.Time {
	t := now.MustParse(str)
	return &t
}

func DialectHasTzSupport() bool {
	// NB: mssql and FoundationDB do not support time zones.
	if dialect := os.Getenv("GORM_DIALECT"); dialect == "mssql" || dialect == "foundation" {
		return false
	}
	return true
}

func OpenTestConnection() (db *gorm.DBCon, err error) {
	switch os.Getenv("GORM_DIALECT") {
	case "mysql":
		// CREATE USER 'gorm'@'localhost' IDENTIFIED BY 'gorm';
		// CREATE DATABASE gorm;
		// GRANT ALL ON * TO 'gorm'@'localhost';
		fmt.Println("testing mysql...")
		dbhost := os.Getenv("GORM_DBADDRESS")
		if dbhost != "" {
			dbhost = fmt.Sprintf("tcp(%v)", dbhost)
		}
		db, err = gorm.Open("mysql", fmt.Sprintf("gorm:gorm@%v/gorm?charset=utf8&parseTime=True", dbhost))
	case "postgres":
		fmt.Println("testing postgres...")
		dbhost := os.Getenv("GORM_DBHOST")
		if dbhost != "" {
			dbhost = fmt.Sprintf("host=%v ", dbhost)
		}
		db, err = gorm.Open("postgres", fmt.Sprintf("%vuser=gorm password=gorm DB.name=gorm sslmode=disable", dbhost))
	case "foundation":
		fmt.Println("testing foundation...")
		db, err = gorm.Open("foundation", "dbname=gorm port=15432 sslmode=disable")
	case "mssql":
		fmt.Println("testing mssql...")
		db, err = gorm.Open("mssql", "server=SERVER_HERE;database=rogue;user id=USER_HERE;password=PW_HERE;port=1433")
	default:
		fmt.Println("testing sqlite3...")
		db, err = gorm.Open("sqlite3", "test.db?cache=shared&mode=memory")
	}

	// db.SetLogger(Logger{log.New(os.Stdout, "\r\n", 0)})
	// db.SetLogger(log.New(os.Stdout, "\r\n", 0))
	if os.Getenv("DEBUG") == "true" {
		db.LogMode(true)
	}

	db.DB().SetMaxIdleConns(10)

	return
}

func TestStringPrimaryKey(t *testing.T) {
	t.Log("1) TestStringPrimaryKey")
	type UUIDStruct struct {
		ID   string `gorm:"primary_key"`
		Name string
	}
	TestDB.DropTable(&UUIDStruct{})
	TestDB.AutoMigrate(&UUIDStruct{})

	data := UUIDStruct{ID: "uuid", Name: "hello"}
	if err := TestDB.Save(&data).Error; err != nil || data.ID != "uuid" || data.Name != "hello" {
		t.Errorf("string primary key should not be populated")
	}

	data = UUIDStruct{ID: "uuid", Name: "hello world"}
	if err := TestDB.Save(&data).Error; err != nil || data.ID != "uuid" || data.Name != "hello world" {
		t.Errorf("string primary key should not be populated")
	}
}

func TestExceptionsWithInvalidSql(t *testing.T) {
	t.Log("2) TestExceptionsWithInvalidSql")
	var columns []string
	if TestDB.Where("sdsd.zaaa = ?", "sd;;;aa").Pluck("aaa", &columns).Error == nil {
		t.Errorf("Should got error with invalid SQL")
	}

	if TestDB.Model(&User{}).Where("sdsd.zaaa = ?", "sd;;;aa").Pluck("aaa", &columns).Error == nil {
		t.Errorf("Should got error with invalid SQL")
	}

	if TestDB.Where("sdsd.zaaa = ?", "sd;;;aa").Find(&User{}).Error == nil {
		t.Errorf("Should got error with invalid SQL")
	}

	var count1, count2 int64
	TestDB.Model(&User{}).Count(&count1)
	if count1 <= 0 {
		t.Errorf("Should find some users")
	}

	if TestDB.Where("name = ?", "jinzhu; delete * from users").First(&User{}).Error == nil {
		t.Errorf("Should got error with invalid SQL")
	}

	TestDB.Model(&User{}).Count(&count2)
	if count1 != count2 {
		t.Errorf("No user should not be deleted by invalid SQL")
	}
}

func TestSetTable(t *testing.T) {
	t.Log("3) TestSetTable")
	TestDB.Create(getPreparedUser("pluck_user1", "pluck_user"))
	TestDB.Create(getPreparedUser("pluck_user2", "pluck_user"))
	TestDB.Create(getPreparedUser("pluck_user3", "pluck_user"))

	if err := TestDB.Table("users").Where("role = ?", "pluck_user").Pluck("age", &[]int{}).Error; err != nil {
		t.Error("No errors should happen if set table for pluck", err)
	}

	var users []User
	if TestDB.Table("users").Find(&[]User{}).Error != nil {
		t.Errorf("No errors should happen if set table for find")
	}

	if TestDB.Table("invalid_table").Find(&users).Error == nil {
		t.Errorf("Should got error when table is set to an invalid table")
	}

	TestDB.Exec("drop table deleted_users;")
	if TestDB.Table("deleted_users").CreateTable(&User{}).Error != nil {
		t.Errorf("Create table with specified table")
	}

	TestDB.Table("deleted_users").Save(&User{Name: "DeletedUser"})

	var deletedUsers []User
	TestDB.Table("deleted_users").Find(&deletedUsers)
	if len(deletedUsers) != 1 {
		t.Errorf("Query from specified table")
	}

	TestDB.Save(getPreparedUser("normal_user", "reset_table"))
	TestDB.Table("deleted_users").Save(getPreparedUser("deleted_user", "reset_table"))
	var user1, user2, user3 User
	TestDB.Where("role = ?", "reset_table").First(&user1).Table("deleted_users").First(&user2).Table("").First(&user3)
	//TODO : @Badu - simplify
	if (user1.Name != "normal_user") || (user2.Name != "deleted_user") || (user3.Name != "normal_user") {
		t.Errorf("unset specified table with blank string")
	}
}

func TestHasTable(t *testing.T) {
	t.Log("4) TestHasTable")
	type Foo struct {
		Id    int
		Stuff string
	}
	TestDB.DropTable(&Foo{})

	// Table should not exist at this point, HasTable should return false
	if ok := TestDB.HasTable("foos"); ok {
		t.Errorf("Table should not exist, but does")
	}
	if ok := TestDB.HasTable(&Foo{}); ok {
		t.Errorf("Table should not exist, but does")
	}

	// We create the table
	if err := TestDB.CreateTable(&Foo{}).Error; err != nil {
		t.Errorf("Table should be created")
	}

	// And now it should exits, and HasTable should return true
	if ok := TestDB.HasTable("foos"); !ok {
		t.Errorf("Table should exist, but HasTable informs it does not")
	}
	if ok := TestDB.HasTable(&Foo{}); !ok {
		t.Errorf("Table should exist, but HasTable informs it does not")
	}
}

func TestTableName(t *testing.T) {
	t.Log("5) TestTableName")
	DB := TestDB.Model("")
	if DB.NewScope(Order{}).TableName() != "orders" {
		t.Errorf("Order's table name should be orders")
	}

	if DB.NewScope(&Order{}).TableName() != "orders" {
		t.Errorf("&Order's table name should be orders")
	}

	if DB.NewScope([]Order{}).TableName() != "orders" {
		t.Errorf("[]Order's table name should be orders")
	}

	if DB.NewScope(&[]Order{}).TableName() != "orders" {
		t.Errorf("&[]Order's table name should be orders")
	}

	DB.SingularTable(true)
	if DB.NewScope(Order{}).TableName() != "order" {
		t.Errorf("Order's singular table name should be order")
	}

	if DB.NewScope(&Order{}).TableName() != "order" {
		t.Errorf("&Order's singular table name should be order")
	}

	if DB.NewScope([]Order{}).TableName() != "order" {
		t.Errorf("[]Order's singular table name should be order")
	}

	if DB.NewScope(&[]Order{}).TableName() != "order" {
		t.Errorf("&[]Order's singular table name should be order")
	}

	if DB.NewScope(&Cart{}).TableName() != "shopping_cart" {
		t.Errorf("&Cart's singular table name should be shopping_cart")
	}

	if DB.NewScope(Cart{}).TableName() != "shopping_cart" {
		t.Errorf("Cart's singular table name should be shopping_cart")
	}

	if DB.NewScope(&[]Cart{}).TableName() != "shopping_cart" {
		t.Errorf("&[]Cart's singular table name should be shopping_cart")
	}

	if DB.NewScope([]Cart{}).TableName() != "shopping_cart" {
		t.Errorf("[]Cart's singular table name should be shopping_cart")
	}
	DB.SingularTable(false)
}

func TestNullValues(t *testing.T) {
	t.Log("6) TestNullValues")
	TestDB.DropTable(&NullValue{})
	TestDB.AutoMigrate(&NullValue{})

	if err := TestDB.Save(&NullValue{
		Name:    sql.NullString{String: "hello", Valid: true},
		Gender:  &sql.NullString{String: "M", Valid: true},
		Age:     sql.NullInt64{Int64: 18, Valid: true},
		Male:    sql.NullBool{Bool: true, Valid: true},
		Height:  sql.NullFloat64{Float64: 100.11, Valid: true},
		AddedAt: NullTime{Time: time.Now(), Valid: true},
	}).Error; err != nil {
		t.Errorf("Not error should raise when test null value")
	}

	var nv NullValue
	TestDB.First(&nv, "name = ?", "hello")

	if nv.Name.String != "hello" || nv.Gender.String != "M" || nv.Age.Int64 != 18 || nv.Male.Bool != true || nv.Height.Float64 != 100.11 || nv.AddedAt.Valid != true {
		t.Errorf("Should be able to fetch null value")
	}

	if err := TestDB.Save(&NullValue{
		Name:    sql.NullString{String: "hello-2", Valid: true},
		Gender:  &sql.NullString{String: "F", Valid: true},
		Age:     sql.NullInt64{Int64: 18, Valid: false},
		Male:    sql.NullBool{Bool: true, Valid: true},
		Height:  sql.NullFloat64{Float64: 100.11, Valid: true},
		AddedAt: NullTime{Time: time.Now(), Valid: false},
	}).Error; err != nil {
		t.Errorf("Not error should raise when test null value")
	}

	var nv2 NullValue
	TestDB.First(&nv2, "name = ?", "hello-2")
	if nv2.Name.String != "hello-2" || nv2.Gender.String != "F" || nv2.Age.Int64 != 0 || nv2.Male.Bool != true || nv2.Height.Float64 != 100.11 || nv2.AddedAt.Valid != false {
		t.Errorf("Should be able to fetch null value")
	}

	if err := TestDB.Save(&NullValue{
		Name:    sql.NullString{String: "hello-3", Valid: false},
		Gender:  &sql.NullString{String: "M", Valid: true},
		Age:     sql.NullInt64{Int64: 18, Valid: false},
		Male:    sql.NullBool{Bool: true, Valid: true},
		Height:  sql.NullFloat64{Float64: 100.11, Valid: true},
		AddedAt: NullTime{Time: time.Now(), Valid: false},
	}).Error; err == nil {
		t.Errorf("Can't save because of name can't be null")
	}
}

func TestNullValuesWithFirstOrCreate(t *testing.T) {
	t.Log("7) TestNullValuesWithFirstOrCreate")
	var nv1 = NullValue{
		Name:   sql.NullString{String: "first_or_create", Valid: true},
		Gender: &sql.NullString{String: "M", Valid: true},
	}

	var nv2 NullValue
	result := TestDB.Where(nv1).FirstOrCreate(&nv2)

	if result.RowsAffected != 1 {
		t.Errorf("RowsAffected should be 1 after create some record")
	}

	if result.Error != nil {
		t.Errorf("Should not raise any error, but got %v", result.Error)
	}

	if nv2.Name.String != "first_or_create" || nv2.Gender.String != "M" {
		t.Errorf("first or create with nullvalues")
	}

	if err := TestDB.Where(nv1).Assign(NullValue{Age: sql.NullInt64{Int64: 18, Valid: true}}).FirstOrCreate(&nv2).Error; err != nil {
		t.Errorf("Should not raise any error, but got %v", err)
	}

	if nv2.Age.Int64 != 18 {
		t.Errorf("should update age to 18")
	}
}

func TestTransaction(t *testing.T) {
	t.Log("8) TestTransaction")
	tx := TestDB.Begin()
	u := User{Name: "transcation"}
	if err := tx.Save(&u).Error; err != nil {
		t.Errorf("No error should raise")
	}

	if err := tx.First(&User{}, "name = ?", "transcation").Error; err != nil {
		t.Errorf("Should find saved record")
	}

	if sqlTx, ok := tx.CommonDB().(*sql.Tx); !ok || sqlTx == nil {
		t.Errorf("Should return the underlying sql.Tx")
	}

	tx.Rollback()

	if err := tx.First(&User{}, "name = ?", "transcation").Error; err == nil {
		t.Errorf("Should not find record after rollback")
	}

	tx2 := TestDB.Begin()
	u2 := User{Name: "transcation-2"}
	if err := tx2.Save(&u2).Error; err != nil {
		t.Errorf("No error should raise")
	}

	if err := tx2.First(&User{}, "name = ?", "transcation-2").Error; err != nil {
		t.Errorf("Should find saved record")
	}

	tx2.Commit()

	if err := TestDB.First(&User{}, "name = ?", "transcation-2").Error; err != nil {
		t.Errorf("Should be able to find committed record")
	}
}

func TestRow(t *testing.T) {
	t.Log("9) TestRow")
	user1 := User{Name: "RowUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "RowUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "RowUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	row := TestDB.Table("users").Where("name = ?", user2.Name).Select("age").Row()
	var age int64
	row.Scan(&age)
	if age != 10 {
		t.Errorf("Scan with Row")
	}
}

func TestRows(t *testing.T) {
	t.Log("10) TestRows")
	user1 := User{Name: "RowsUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "RowsUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "RowsUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	rows, err := TestDB.Table("users").Where("name = ? or name = ?", user2.Name, user3.Name).Select("name, age").Rows()
	if err != nil {
		t.Errorf("Not error should happen, got %v", err)
	}

	count := 0
	for rows.Next() {
		var name string
		var age int64
		rows.Scan(&name, &age)
		count++
	}

	if count != 2 {
		t.Errorf("Should found two records")
	}
}

func TestScanRows(t *testing.T) {
	t.Log("11) TestScanRows")
	user1 := User{Name: "ScanRowsUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "ScanRowsUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "ScanRowsUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	rows, err := TestDB.Table("users").Where("name = ? or name = ?", user2.Name, user3.Name).Select("name, age").Rows()
	if err != nil {
		t.Errorf("Not error should happen, got %v", err)
	}

	type Result struct {
		Name string
		Age  int
	}

	var results []Result
	for rows.Next() {
		var result Result
		if err := TestDB.ScanRows(rows, &result); err != nil {
			t.Errorf("should get no error, but got %v", err)
		}
		results = append(results, result)
	}

	if !reflect.DeepEqual(results, []Result{{Name: "ScanRowsUser2", Age: 10}, {Name: "ScanRowsUser3", Age: 20}}) {
		t.Errorf("Should find expected results")
	}
}

func TestScan(t *testing.T) {
	t.Log("12) TestScan")
	user1 := User{Name: "ScanUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "ScanUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "ScanUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	type result struct {
		Name string
		Age  int
	}

	var res result
	TestDB.Table("users").Select("name, age").Where("name = ?", user3.Name).Scan(&res)
	if res.Name != user3.Name {
		t.Errorf("Scan into struct should work")
	}

	var doubleAgeRes result
	TestDB.Table("users").Select("age + age as age").Where("name = ?", user3.Name).Scan(&doubleAgeRes)
	if doubleAgeRes.Age != res.Age*2 {
		t.Errorf("Scan double age as age")
	}

	var ress []result
	TestDB.Table("users").Select("name, age").Where("name in (?)", []string{user2.Name, user3.Name}).Scan(&ress)
	if len(ress) != 2 || ress[0].Name != user2.Name || ress[1].Name != user3.Name {
		t.Errorf("Scan into struct map")
	}
}

func TestRaw(t *testing.T) {
	t.Log("13) TestRaw")
	user1 := User{Name: "ExecRawSqlUser1", Age: 1, Birthday: parseTime("2000-1-1")}
	user2 := User{Name: "ExecRawSqlUser2", Age: 10, Birthday: parseTime("2010-1-1")}
	user3 := User{Name: "ExecRawSqlUser3", Age: 20, Birthday: parseTime("2020-1-1")}
	TestDB.Save(&user1).Save(&user2).Save(&user3)

	type result struct {
		Name  string
		Email string
	}

	var ress []result
	TestDB.Raw("SELECT name, age FROM users WHERE name = ? or name = ?", user2.Name, user3.Name).Scan(&ress)
	if len(ress) != 2 || ress[0].Name != user2.Name || ress[1].Name != user3.Name {
		t.Errorf("Raw with scan")
	}

	rows, _ := TestDB.Raw("select name, age from users where name = ?", user3.Name).Rows()
	count := 0
	for rows.Next() {
		count++
	}
	if count != 1 {
		t.Errorf("Raw with Rows should find one record with name 3")
	}

	TestDB.Exec("update users set name=? where name in (?)", "jinzhu", []string{user1.Name, user2.Name, user3.Name})
	if TestDB.Where("name in (?)", []string{user1.Name, user2.Name, user3.Name}).First(&User{}).Error != gorm.ErrRecordNotFound {
		t.Error("Raw sql to update records")
	}
}

func TestGroup(t *testing.T) {
	t.Log("14) TestGroup")
	rows, err := TestDB.Select("name").Table("users").Group("name").Rows()

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)
		}
	} else {
		t.Errorf("Should not raise any error")
	}
}

func TestJoins(t *testing.T) {
	t.Log("15) TestJoins")
	var user = User{
		Name:       "joins",
		CreditCard: CreditCard{Number: "411111111111"},
		Emails:     []Email{{Email: "join1@example.com"}, {Email: "join2@example.com"}},
	}
	TestDB.Save(&user)

	var users1 []User
	TestDB.Joins("left join emails on emails.user_id = users.id").Where("name = ?", "joins").Find(&users1)
	if len(users1) != 2 {
		t.Errorf("should find two users using left join")
	}

	var users2 []User
	TestDB.Joins("left join emails on emails.user_id = users.id AND emails.email = ?", "join1@example.com").Where("name = ?", "joins").First(&users2)
	if len(users2) != 1 {
		t.Errorf("should find one users using left join with conditions")
	}

	var users3 []User
	TestDB.Joins("join emails on emails.user_id = users.id AND emails.email = ?", "join1@example.com").Joins("join credit_cards on credit_cards.user_id = users.id AND credit_cards.number = ?", "411111111111").Where("name = ?", "joins").First(&users3)
	if len(users3) != 1 {
		t.Errorf("should find one users using multiple left join conditions")
	}

	var users4 []User
	TestDB.Joins("join emails on emails.user_id = users.id AND emails.email = ?", "join1@example.com").Joins("join credit_cards on credit_cards.user_id = users.id AND credit_cards.number = ?", "422222222222").Where("name = ?", "joins").First(&users4)
	if len(users4) != 0 {
		t.Errorf("should find no user when searching with unexisting credit card")
	}

	var users5 []User
	db5 := TestDB.Joins("join emails on emails.user_id = users.id AND emails.email = ?", "join1@example.com").Joins("join credit_cards on credit_cards.user_id = users.id AND credit_cards.number = ?", "411111111111").Where(User{Id: 1}).Where(Email{Id: 1}).Not(Email{Id: 10}).First(&users5)
	if db5.Error != nil {
		t.Errorf("Should not raise error for join where identical fields in different tables. Error: %s", db5.Error.Error())
	}
}

func TestJoinsWithSelect(t *testing.T) {
	t.Log("16) TestJoinsWithSelect")
	type result struct {
		Name  string
		Email string
	}

	user := User{
		Name:   "joins_with_select",
		Emails: []Email{{Email: "join1@example.com"}, {Email: "join2@example.com"}},
	}
	TestDB.Save(&user)

	var results []result
	TestDB.Table("users").Select("name, emails.email").Joins("left join emails on emails.user_id = users.id").Where("name = ?", "joins_with_select").Scan(&results)
	if len(results) != 2 || results[0].Email != "join1@example.com" || results[1].Email != "join2@example.com" {
		t.Errorf("Should find all two emails with Join select")
	}
}

func TestHaving(t *testing.T) {
	t.Log("17) TestHaving")
	rows, err := TestDB.Select("name, count(*) as total").Table("users").Group("name").Having("name IN (?)", []string{"2", "3"}).Rows()

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			var total int64
			rows.Scan(&name, &total)

			if name == "2" && total != 1 {
				t.Errorf("Should have one user having name 2")
			}
			if name == "3" && total != 2 {
				t.Errorf("Should have two users having name 3")
			}
		}
	} else {
		t.Errorf("Should not raise any error")
	}
}

func TestTimeWithZone(t *testing.T) {
	t.Log("18) TestTimeWithZone")
	var format = "2006-01-02 15:04:05 -0700"
	var times []time.Time
	GMT8, _ := time.LoadLocation("Asia/Shanghai")
	times = append(times, time.Date(2013, 02, 19, 1, 51, 49, 123456789, GMT8))
	times = append(times, time.Date(2013, 02, 18, 17, 51, 49, 123456789, time.UTC))

	for index, vtime := range times {
		name := "time_with_zone_" + strconv.Itoa(index)
		user := User{Name: name, Birthday: &vtime}

		if !DialectHasTzSupport() {
			// If our driver dialect doesn't support TZ's, just use UTC for everything here.
			utcBirthday := user.Birthday.UTC()
			user.Birthday = &utcBirthday
		}

		TestDB.Save(&user)
		expectedBirthday := "2013-02-18 17:51:49 +0000"
		foundBirthday := user.Birthday.UTC().Format(format)
		if foundBirthday != expectedBirthday {
			t.Errorf("User's birthday should not be changed after save for name=%s, expected bday=%+v but actual value=%+v", name, expectedBirthday, foundBirthday)
		}

		var findUser, findUser2, findUser3 User
		TestDB.First(&findUser, "name = ?", name)
		foundBirthday = findUser.Birthday.UTC().Format(format)
		if foundBirthday != expectedBirthday {
			t.Errorf("User's birthday should not be changed after find for name=%s, expected bday=%+v but actual value=%+v", name, expectedBirthday, foundBirthday)
		}

		if TestDB.Where("id = ? AND birthday >= ?", findUser.Id, user.Birthday.Add(-time.Minute)).First(&findUser2).RecordNotFound() {
			t.Errorf("User should be found")
		}

		if !TestDB.Where("id = ? AND birthday >= ?", findUser.Id, user.Birthday.Add(time.Minute)).First(&findUser3).RecordNotFound() {
			t.Errorf("User should not be found")
		}
	}
}

func TestHstore(t *testing.T) {
	t.Log("19) TestHstore")
	type Details struct {
		Id   int64
		Bulk pgdialect.Hstore
	}

	if dialect := os.Getenv("GORM_DIALECT"); dialect != "postgres" {
		t.Skip()
	}

	if err := TestDB.Exec("CREATE EXTENSION IF NOT EXISTS hstore").Error; err != nil {
		fmt.Println("\033[31mHINT: Must be superuser to create hstore extension (ALTER USER gorm WITH SUPERUSER;)\033[0m")
		panic(fmt.Sprintf("No error should happen when create hstore extension, but got %+v", err))
	}

	TestDB.Exec("drop table details")

	if err := TestDB.CreateTable(&Details{}).Error; err != nil {
		panic(fmt.Sprintf("No error should happen when create table, but got %+v", err))
	}

	bankAccountId, phoneNumber, opinion := "123456", "14151321232", "sharkbait"
	bulk := map[string]*string{
		"bankAccountId": &bankAccountId,
		"phoneNumber":   &phoneNumber,
		"opinion":       &opinion,
	}
	d := Details{Bulk: bulk}
	TestDB.Save(&d)

	var d2 Details
	if err := TestDB.First(&d2).Error; err != nil {
		t.Errorf("Got error when tried to fetch details: %+v", err)
	}

	for k := range bulk {
		if r, ok := d2.Bulk[k]; ok {
			if res, _ := bulk[k]; *res != *r {
				t.Errorf("Details should be equal")
			}
		} else {
			t.Errorf("Details should be existed")
		}
	}
}

func TestSetAndGet(t *testing.T) {
	t.Log("20) TestSetAndGet")
	if value, ok := TestDB.Set("hello", "world").Get("hello"); !ok {
		t.Errorf("Should be able to get setting after set")
	} else {
		if value.(string) != "world" {
			t.Errorf("Setted value should not be changed")
		}
	}

	if _, ok := TestDB.Get("non_existing"); ok {
		t.Errorf("Get non existing key should return error")
	}
}

func TestCompatibilityMode(t *testing.T) {
	t.Log("21) TestCompatibilityMode")
	DB, _ := gorm.Open("testdb", "")
	testdb.SetQueryFunc(func(query string) (driver.Rows, error) {
		columns := []string{"id", "name", "age"}
		result := `
		1,Tim,20
		2,Joe,25
		3,Bob,30
		`
		return testdb.RowsFromCSVString(columns, result), nil
	})

	var users []User
	DB.Find(&users)
	if (users[0].Name != "Tim") || len(users) != 3 {
		t.Errorf("Unexcepted result returned")
	}
}

func TestOpenExistingDB(t *testing.T) {
	t.Log("22) TestOpenExistingDB")
	TestDB.Save(&User{Name: "jnfeinstein"})
	dialect := os.Getenv("GORM_DIALECT")

	db, err := gorm.Open(dialect, TestDB.DB())
	if err != nil {
		t.Errorf("Should have wrapped the existing DB connection")
	}

	var user User
	if db.Where("name = ?", "jnfeinstein").First(&user).Error == gorm.ErrRecordNotFound {
		t.Errorf("Should have found existing record")
	}
}

func TestDdlErrors(t *testing.T) {
	t.Log("23) TestDdlErrors")
	var err error

	if err = TestDB.Close(); err != nil {
		t.Errorf("Closing DDL test db connection err=%s", err)
	}
	defer func() {
		// Reopen DB connection.
		if TestDB, err = OpenTestConnection(); err != nil {
			t.Fatalf("Failed re-opening db connection: %s", err)
		}
	}()

	if err := TestDB.Find(&User{}).Error; err == nil {
		t.Errorf("Expected operation on closed db to produce an error, but err was nil")
	}
}

func TestOpenWithOneParameter(t *testing.T) {
	t.Log("24) TestOpenWithOneParameter")
	db, err := gorm.Open("dialect")
	if db != nil {
		t.Error("Open with one parameter returned non nil for db")
	}
	if err == nil {
		t.Error("Open with one parameter returned err as nil")
	}
}

func BenchmarkGorm(b *testing.B) {
	b.Log("1) BenchmarkGorm")
	b.N = 2000
	for x := 0; x < b.N; x++ {
		e := strconv.Itoa(x) + "benchmark@example.org"
		now := time.Now()
		email := BigEmail{Email: e, UserAgent: "pc", RegisteredAt: &now}
		// Insert
		TestDB.Save(&email)
		// Query
		TestDB.First(&BigEmail{}, "email = ?", e)
		// Update
		TestDB.Model(&email).UpdateColumn("email", "new-"+e)
		// Delete
		TestDB.Delete(&email)
	}
}

func BenchmarkRawSql(b *testing.B) {
	b.Log("2) BenchmarkRawSql")
	DB, _ := sql.Open("postgres", "user=gorm DB.ame=gorm sslmode=disable")
	DB.SetMaxIdleConns(10)
	insertSql := "INSERT INTO emails (user_id,email,user_agent,registered_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id"
	querySql := "SELECT * FROM emails WHERE email = $1 ORDER BY id LIMIT 1"
	updateSql := "UPDATE emails SET email = $1, updated_at = $2 WHERE id = $3"
	deleteSql := "DELETE FROM orders WHERE id = $1"

	b.N = 2000
	for x := 0; x < b.N; x++ {
		var id int64
		e := strconv.Itoa(x) + "benchmark@example.org"
		now := time.Now()
		email := BigEmail{Email: e, UserAgent: "pc", RegisteredAt: &now}
		// Insert
		DB.QueryRow(insertSql, email.UserId, email.Email, email.UserAgent, email.RegisteredAt, time.Now(), time.Now()).Scan(&id)
		// Query
		rows, _ := DB.Query(querySql, email.Email)
		rows.Close()
		// Update
		DB.Exec(updateSql, "new-"+e, time.Now(), id)
		// Delete
		DB.Exec(deleteSql, id)
	}
}
