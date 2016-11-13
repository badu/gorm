package tests

import (
	"testing"
	"time"
)

func TestCustomizeColumn(t *testing.T) {
	t.Log("55) TestCustomizeColumn")
	col := "mapped_name"
	TestDB.DropTable(&CustomizeColumn{})
	TestDB.AutoMigrate(&CustomizeColumn{})

	scope := TestDB.NewScope(&CustomizeColumn{})
	if !scope.Dialect().HasColumn(scope.TableName(), col) {
		t.Errorf("CustomizeColumn should have column %s", col)
	}

	col = "mapped_id"
	if scope.PKName() != col {
		t.Errorf("CustomizeColumn should have primary key %s, but got %q", col, scope.PKName())
	}

	expected := "foo"
	now := time.Now()
	cc := CustomizeColumn{ID: 666, Name: expected, Date: &now}

	if count := TestDB.Create(&cc).RowsAffected; count != 1 {
		t.Error("There should be one record be affected when create record")
	}

	var cc1 CustomizeColumn
	TestDB.First(&cc1, 666)

	if cc1.Name != expected {
		t.Errorf("Failed to query CustomizeColumn")
	}

	cc.Name = "bar"
	TestDB.Save(&cc)

	var cc2 CustomizeColumn
	TestDB.First(&cc2, 666)
	if cc2.Name != "bar" {
		t.Errorf("Failed to query CustomizeColumn")
	}
}

func TestCustomColumnAndIgnoredFieldClash(t *testing.T) {
	t.Log("56) TestCustomColumnAndIgnoredFieldClash")
	TestDB.DropTable(&CustomColumnAndIgnoredFieldClash{})
	if err := TestDB.AutoMigrate(&CustomColumnAndIgnoredFieldClash{}).Error; err != nil {
		t.Errorf("Should not raise error: %s", err)
	}
}

func TestManyToManyWithCustomizedColumn(t *testing.T) {
	t.Log("57) TestManyToManyWithCustomizedColumn")
	TestDB.DropTable(&CustomizePerson{}, &CustomizeAccount{}, "PersonAccount")
	TestDB.AutoMigrate(&CustomizePerson{}, &CustomizeAccount{})

	account := CustomizeAccount{IdAccount: "account", Name: "id1"}
	person := CustomizePerson{
		IdPerson: "person",
		Accounts: []CustomizeAccount{account},
	}

	if err := TestDB.Create(&account).Error; err != nil {
		t.Errorf("no error should happen, but got %v", err)
	}

	if err := TestDB.Create(&person).Error; err != nil {
		t.Errorf("no error should happen, but got %v", err)
	}

	var person1 CustomizePerson
	scope := TestDB.NewScope(nil)
	if err := TestDB.Preload("Accounts").First(&person1, scope.Quote("idPerson")+" = ?", person.IdPerson).Error; err != nil {
		t.Errorf("no error should happen when preloading customized column many2many relations, but got %v", err)
	}

	if len(person1.Accounts) != 1 || person1.Accounts[0].IdAccount != "account" {
		t.Errorf("should preload correct accounts")
	}
}

func TestOneToOneWithCustomizedColumn(t *testing.T) {
	t.Log("58) TestOneToOneWithCustomizedColumn")
	TestDB.DropTable(&CustomizeUser{}, &CustomizeInvitation{})
	TestDB.AutoMigrate(&CustomizeUser{}, &CustomizeInvitation{})

	user := CustomizeUser{
		Email: "hello@example.com",
	}
	invitation := CustomizeInvitation{
		Address: "hello@example.com",
	}

	TestDB.Create(&user)
	TestDB.Create(&invitation)

	var invitation2 CustomizeInvitation
	if err := TestDB.Preload("Person").Find(&invitation2, invitation.ID).Error; err != nil {
		t.Errorf("no error should happen, but got %v", err)
	}

	if invitation2.Person.Email != user.Email {
		t.Errorf("Should preload one to one relation with customize foreign keys")
	}
}

func TestOneToManyWithCustomizedColumn(t *testing.T) {
	t.Log("59) TestOneToManyWithCustomizedColumn")
	TestDB.DropTable(&PromotionDiscount{}, &PromotionCoupon{})
	TestDB.AutoMigrate(&PromotionDiscount{}, &PromotionCoupon{})

	discount := PromotionDiscount{
		Name: "Happy New Year",
		Coupons: []*PromotionCoupon{
			{Code: "newyear1"},
			{Code: "newyear2"},
		},
	}

	if err := TestDB.Create(&discount).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	var discount1 PromotionDiscount
	if err := TestDB.Preload("Coupons").First(&discount1, "id = ?", discount.ID).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if len(discount.Coupons) != 2 {
		t.Errorf("should find two coupons")
	}

	var coupon PromotionCoupon
	if err := TestDB.Preload("Discount").First(&coupon, "code = ?", "newyear1").Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if coupon.Discount.Name != "Happy New Year" {
		t.Errorf("should preload discount from coupon")
	}
}

func TestHasOneWithPartialCustomizedColumn(t *testing.T) {
	t.Log("60) TestHasOneWithPartialCustomizedColumn")
	TestDB.DropTable(&PromotionDiscount{}, &PromotionRule{})
	TestDB.AutoMigrate(&PromotionDiscount{}, &PromotionRule{})

	var begin = time.Now()
	var end = time.Now().Add(24 * time.Hour)
	discount := PromotionDiscount{
		Name: "Happy New Year 2",
		Rule: &PromotionRule{
			Name:  "time_limited",
			Begin: &begin,
			End:   &end,
		},
	}

	if err := TestDB.Create(&discount).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	var discount1 PromotionDiscount
	if err := TestDB.Preload("Rule").First(&discount1, "id = ?", discount.ID).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if discount.Rule.Begin.Format(time.RFC3339Nano) != begin.Format(time.RFC3339Nano) {
		t.Errorf("Should be able to preload Rule")
	}

	var rule PromotionRule
	if err := TestDB.Preload("Discount").First(&rule, "name = ?", "time_limited").Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if rule.Discount.Name != "Happy New Year 2" {
		t.Errorf("should preload discount from rule")
	}
}

func TestBelongsToWithPartialCustomizedColumn(t *testing.T) {
	t.Log("61) TestBelongsToWithPartialCustomizedColumn")
	TestDB.DropTable(&PromotionDiscount{}, &PromotionBenefit{})
	TestDB.AutoMigrate(&PromotionDiscount{}, &PromotionBenefit{})

	discount := PromotionDiscount{
		Name: "Happy New Year 3",
		Benefits: []PromotionBenefit{
			{Name: "free cod"},
			{Name: "free shipping"},
		},
	}

	if err := TestDB.Create(&discount).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	var discount1 PromotionDiscount
	if err := TestDB.Preload("Benefits").First(&discount1, "id = ?", discount.ID).Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if len(discount.Benefits) != 2 {
		t.Errorf("should find two benefits")
	}

	var benefit PromotionBenefit
	if err := TestDB.Preload("Discount").First(&benefit, "name = ?", "free cod").Error; err != nil {
		t.Errorf("no error should happen but got %v", err)
	}

	if benefit.Discount.Name != "Happy New Year 3" {
		t.Errorf("should preload discount from coupon")
	}
}
