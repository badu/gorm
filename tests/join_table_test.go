package tests

import (
	"testing"
)

func DoJoinTable(t *testing.T) {

	TestDB.Exec("drop table person_addresses;")
	TestDB.AutoMigrate(&Person{}, &PersonAddress{})
	TestDB.SetJoinTableHandler(&Person{}, "Addresses", &PersonAddress{})

	address1 := &Address{Address1: "address 1 of person 1"}
	address2 := &Address{Address1: "address 2 of person 1"}
	person := &Person{Name: "person which is people because ?", Addresses: []*Address{address1, address2}}
	TestDB.Save(person)
	person2 := &Person{}

	TestDB.Model(Person{}).Related(&person2.Addresses, "Addresses").Where(Person{Id: person.Id}).Find(&person2)

	//t.Logf("%s", person2)
	TestDB.Model(person).Association("Addresses").Delete(address1)

	if TestDB.Find(&[]PersonAddress{}, "person_id = ?", person.Id).RowsAffected != 1 {
		t.Errorf("Should found one address")
	}

	if TestDB.Model(person).Association("Addresses").Count() != 1 {
		t.Errorf("Should found one address")
	}

	if TestDB.Unscoped().Find(&[]PersonAddress{}, "person_id = ?", person.Id).RowsAffected != 2 {
		t.Errorf("Found two addresses with Unscoped")
	}

	if TestDB.Model(person).Association("Addresses").Clear(); TestDB.Model(person).Association("Addresses").Count() != 0 {
		t.Errorf("Should deleted all addresses")
	}
}
