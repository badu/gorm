package tests

import (
	"testing"
)

func TestJoinTable(t *testing.T) {
	t.Log("68) TestJoinTable")

	TestDB.Exec("drop table person_addresses;")
	TestDB.AutoMigrate(&Person{})
	TestDB.SetJoinTableHandler(&Person{}, "Addresses", &PersonAddress{})

	address1 := &Address{Address1: "address 1 of person 1"}
	address2 := &Address{Address1: "address 2 of person 1"}
	person := &Person{Name: "person", Addresses: []*Address{address1, address2}}
	TestDB.Save(person)
	person2 := &Person{}
	TestDB.Model(person).Where(Person{Id: person.Id}).Related(&person2.Addresses, "Addresses").Find(&person2)
	//TODO : @Badu - seems to me it fails retrieving with relations as I expect it
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
