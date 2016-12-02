package tests

import (
	"testing"
)

func Polymorphic(t *testing.T) {
	cat := Cat{Name: "Mr. Bigglesworth", Toy: Toy{Name: "cat toy"}}
	dog := Dog{Name: "Pluto", Toys: []Toy{{Name: "dog toy 1"}, {Name: "dog toy 2"}}}
	TestDB.Save(&cat).Save(&dog)

	if TestDB.Model(&cat).Association("Toy").Count() != 1 {
		t.Errorf("Cat's toys count should be 1")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 2 {
		t.Errorf("Dog's toys count should be 2")
	}

	// Query
	var catToys []Toy
	if TestDB.Model(&cat).Related(&catToys, "Toy").RecordNotFound() {
		t.Errorf("Did not find any has one polymorphic association")
	} else if len(catToys) != 1 {
		t.Errorf("Should have found only one polymorphic has one association")
	} else if catToys[0].Name != cat.Toy.Name {
		t.Errorf("Should have found the proper has one polymorphic association")
	}

	var dogToys []Toy
	if TestDB.Model(&dog).Related(&dogToys, "Toys").RecordNotFound() {
		t.Errorf("Did not find any polymorphic has many associations")
	} else if len(dogToys) != len(dog.Toys) {
		t.Errorf("Should have found all polymorphic has many associations")
	}

	var catToy Toy
	TestDB.Model(&cat).Association("Toy").Find(&catToy)
	if catToy.Name != cat.Toy.Name {
		t.Errorf("Should find has one polymorphic association")
	}

	var dogToys1 []Toy
	TestDB.Model(&dog).Association("Toys").Find(&dogToys1)
	if !compareToys(dogToys1, []string{"dog toy 1", "dog toy 2"}) {
		t.Errorf("Should find has many polymorphic association")
	}

	// Append
	TestDB.Model(&cat).Association("Toy").Append(&Toy{
		Name: "cat toy 2",
	})

	var catToy2 Toy
	TestDB.Model(&cat).Association("Toy").Find(&catToy2)
	if catToy2.Name != "cat toy 2" {
		t.Errorf("Should update has one polymorphic association with Append")
	}

	if TestDB.Model(&cat).Association("Toy").Count() != 1 {
		t.Errorf("Cat's toys count should be 1 after Append")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 2 {
		t.Errorf("Should return two polymorphic has many associations")
	}

	TestDB.Model(&dog).Association("Toys").Append(&Toy{
		Name: "dog toy 3",
	})

	var dogToys2 []Toy
	TestDB.Model(&dog).Association("Toys").Find(&dogToys2)
	if !compareToys(dogToys2, []string{"dog toy 1", "dog toy 2", "dog toy 3"}) {
		t.Errorf("Dog's toys should be updated with Append")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 3 {
		t.Errorf("Should return three polymorphic has many associations")
	}

	// Replace
	TestDB.Model(&cat).Association("Toy").Replace(&Toy{
		Name: "cat toy 3",
	})

	var catToy3 Toy
	TestDB.Model(&cat).Association("Toy").Find(&catToy3)
	if catToy3.Name != "cat toy 3" {
		t.Errorf("Should update has one polymorphic association with Replace")
	}

	if TestDB.Model(&cat).Association("Toy").Count() != 1 {
		t.Errorf("Cat's toys count should be 1 after Replace")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 3 {
		t.Errorf("Should return three polymorphic has many associations")
	}

	TestDB.Model(&dog).Association("Toys").Replace(&Toy{
		Name: "dog toy 4",
	}, []Toy{
		{Name: "dog toy 5"}, {Name: "dog toy 6"}, {Name: "dog toy 7"},
	})

	var dogToys3 []Toy
	TestDB.Model(&dog).Association("Toys").Find(&dogToys3)
	if !compareToys(dogToys3, []string{"dog toy 4", "dog toy 5", "dog toy 6", "dog toy 7"}) {
		t.Errorf("Dog's toys should be updated with Replace")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 4 {
		t.Errorf("Should return three polymorphic has many associations")
	}

	// Delete
	TestDB.Model(&cat).Association("Toy").Delete(&catToy2)

	var catToy4 Toy
	TestDB.Model(&cat).Association("Toy").Find(&catToy4)
	if catToy4.Name != "cat toy 3" {
		t.Errorf("Should not update has one polymorphic association when Delete a unrelated Toy")
	}

	if TestDB.Model(&cat).Association("Toy").Count() != 1 {
		t.Errorf("Cat's toys count should be 1")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 4 {
		t.Errorf("Dog's toys count should be 4")
	}

	TestDB.Model(&cat).Association("Toy").Delete(&catToy3)

	if !TestDB.Model(&cat).Related(&Toy{}, "Toy").RecordNotFound() {
		t.Errorf("Toy should be deleted with Delete")
	}

	if TestDB.Model(&cat).Association("Toy").Count() != 0 {
		t.Errorf("Cat's toys count should be 0 after Delete")
	}

	if TestDB.Model(&dog).Association("Toys").Count() != 4 {
		t.Errorf("Dog's toys count should not be changed when delete cat's toy")
	}

	TestDB.Model(&dog).Association("Toys").Delete(&dogToys2)

	if TestDB.Model(&dog).Association("Toys").Count() != 4 {
		t.Errorf("Dog's toys count should not be changed when delete unrelated toys")
	}

	TestDB.Model(&dog).Association("Toys").Delete(&dogToys3)

	if TestDB.Model(&dog).Association("Toys").Count() != 0 {
		t.Errorf("Dog's toys count should be deleted with Delete")
	}

	// Clear
	TestDB.Model(&cat).Association("Toy").Append(&Toy{
		Name: "cat toy 2",
	})

	if TestDB.Model(&cat).Association("Toy").Count() != 1 {
		t.Errorf("Cat's toys should be added with Append")
	}

	TestDB.Model(&cat).Association("Toy").Clear()

	if TestDB.Model(&cat).Association("Toy").Count() != 0 {
		t.Errorf("Cat's toys should be cleared with Clear")
	}

	TestDB.Model(&dog).Association("Toys").Append(&Toy{
		Name: "dog toy 8",
	})

	if TestDB.Model(&dog).Association("Toys").Count() != 1 {
		t.Errorf("Dog's toys should be added with Append")
	}

	TestDB.Model(&dog).Association("Toys").Clear()

	if TestDB.Model(&dog).Association("Toys").Count() != 0 {
		t.Errorf("Dog's toys should be cleared with Clear")
	}
}

func NamedPolymorphic(t *testing.T) {
	hamster := Hamster{Name: "Mr. Hammond", PreferredToy: Toy{Name: "bike"}, OtherToy: Toy{Name: "treadmill"}}
	TestDB.Save(&hamster)

	hamster2 := Hamster{}
	TestDB.Preload("PreferredToy").Preload("OtherToy").Find(&hamster2, hamster.Id)
	if hamster2.PreferredToy.Id != hamster.PreferredToy.Id || hamster2.PreferredToy.Name != hamster.PreferredToy.Name {
		t.Errorf("Hamster's preferred toy couldn't be preloaded")
	}
	if hamster2.OtherToy.Id != hamster.OtherToy.Id || hamster2.OtherToy.Name != hamster.OtherToy.Name {
		t.Errorf("Hamster's other toy couldn't be preloaded")
	}

	// clear to omit Toy.Id in count
	hamster2.PreferredToy = Toy{}
	hamster2.OtherToy = Toy{}

	if TestDB.Model(&hamster2).Association("PreferredToy").Count() != 1 {
		t.Errorf("Hamster's preferred toy count should be 1")
	}

	if TestDB.Model(&hamster2).Association("OtherToy").Count() != 1 {
		t.Errorf("Hamster's other toy count should be 1")
	}

	// Query
	var hamsterToys []Toy
	if TestDB.Model(&hamster).Related(&hamsterToys, "PreferredToy").RecordNotFound() {
		t.Errorf("Did not find any has one polymorphic association")
	} else if len(hamsterToys) != 1 {
		t.Errorf("Should have found only one polymorphic has one association")
	} else if hamsterToys[0].Name != hamster.PreferredToy.Name {
		t.Errorf("Should have found the proper has one polymorphic association")
	}

	if TestDB.Model(&hamster).Related(&hamsterToys, "OtherToy").RecordNotFound() {
		t.Errorf("Did not find any has one polymorphic association")
	} else if len(hamsterToys) != 1 {
		t.Errorf("Should have found only one polymorphic has one association")
	} else if hamsterToys[0].Name != hamster.OtherToy.Name {
		t.Errorf("Should have found the proper has one polymorphic association")
	}

	hamsterToy := Toy{}
	TestDB.Model(&hamster).Association("PreferredToy").Find(&hamsterToy)
	if hamsterToy.Name != hamster.PreferredToy.Name {
		t.Errorf("Should find has one polymorphic association")
	}
	hamsterToy = Toy{}
	TestDB.Model(&hamster).Association("OtherToy").Find(&hamsterToy)
	if hamsterToy.Name != hamster.OtherToy.Name {
		t.Errorf("Should find has one polymorphic association")
	}

	// Append
	TestDB.Model(&hamster).Association("PreferredToy").Append(&Toy{
		Name: "bike 2",
	})
	TestDB.Model(&hamster).Association("OtherToy").Append(&Toy{
		Name: "treadmill 2",
	})

	hamsterToy = Toy{}
	TestDB.Model(&hamster).Association("PreferredToy").Find(&hamsterToy)
	if hamsterToy.Name != "bike 2" {
		t.Errorf("Should update has one polymorphic association with Append")
	}

	hamsterToy = Toy{}
	TestDB.Model(&hamster).Association("OtherToy").Find(&hamsterToy)
	if hamsterToy.Name != "treadmill 2" {
		t.Errorf("Should update has one polymorphic association with Append")
	}

	if TestDB.Model(&hamster2).Association("PreferredToy").Count() != 1 {
		t.Errorf("Hamster's toys count should be 1 after Append")
	}

	if TestDB.Model(&hamster2).Association("OtherToy").Count() != 1 {
		t.Errorf("Hamster's toys count should be 1 after Append")
	}

	// Replace
	TestDB.Model(&hamster).Association("PreferredToy").Replace(&Toy{
		Name: "bike 3",
	})
	TestDB.Model(&hamster).Association("OtherToy").Replace(&Toy{
		Name: "treadmill 3",
	})

	hamsterToy = Toy{}
	TestDB.Model(&hamster).Association("PreferredToy").Find(&hamsterToy)
	if hamsterToy.Name != "bike 3" {
		t.Errorf("Should update has one polymorphic association with Replace")
	}

	hamsterToy = Toy{}
	TestDB.Model(&hamster).Association("OtherToy").Find(&hamsterToy)
	if hamsterToy.Name != "treadmill 3" {
		t.Errorf("Should update has one polymorphic association with Replace")
	}

	if TestDB.Model(&hamster2).Association("PreferredToy").Count() != 1 {
		t.Errorf("hamster's toys count should be 1 after Replace")
	}

	if TestDB.Model(&hamster2).Association("OtherToy").Count() != 1 {
		t.Errorf("hamster's toys count should be 1 after Replace")
	}

	// Clear
	TestDB.Model(&hamster).Association("PreferredToy").Append(&Toy{
		Name: "bike 2",
	})
	TestDB.Model(&hamster).Association("OtherToy").Append(&Toy{
		Name: "treadmill 2",
	})

	if TestDB.Model(&hamster).Association("PreferredToy").Count() != 1 {
		t.Errorf("Hamster's toys should be added with Append")
	}
	if TestDB.Model(&hamster).Association("OtherToy").Count() != 1 {
		t.Errorf("Hamster's toys should be added with Append")
	}

	TestDB.Model(&hamster).Association("PreferredToy").Clear()

	if TestDB.Model(&hamster2).Association("PreferredToy").Count() != 0 {
		t.Errorf("Hamster's preferred toy should be cleared with Clear")
	}
	if TestDB.Model(&hamster2).Association("OtherToy").Count() != 1 {
		t.Errorf("Hamster's other toy should be still available")
	}

	TestDB.Model(&hamster).Association("OtherToy").Clear()
	if TestDB.Model(&hamster).Association("OtherToy").Count() != 0 {
		t.Errorf("Hamster's other toy should be cleared with Clear")
	}
}
