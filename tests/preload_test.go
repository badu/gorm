package tests

import (
	"database/sql"
	"os"
	"reflect"
	"testing"
	"gorm"
)

func TestPreload(t *testing.T) {
	//t.Log("78) TestPreload")
	user1 := getPreloadUser("user1")
	TestDB.Save(user1)

	preloadDB := TestDB.Where("role = ?", "Preload").Preload("BillingAddress").Preload("ShippingAddress").
		Preload("CreditCard").Preload("Emails").Preload("Company")
	var user User
	preloadDB.Find(&user)
	checkUserHasPreloadData(user, t)

	user2 := getPreloadUser("user2")
	TestDB.Save(user2)

	user3 := getPreloadUser("user3")
	TestDB.Save(user3)

	var users []User
	preloadDB.Find(&users)

	for _, user := range users {
		checkUserHasPreloadData(user, t)
	}

	var users2 []*User
	preloadDB.Find(&users2)

	for _, user := range users2 {
		checkUserHasPreloadData(*user, t)
	}

	var users3 []*User
	preloadDB.Preload("Emails", "email = ?", user3.Emails[0].Email).Find(&users3)

	for _, user := range users3 {
		if user.Name == user3.Name {
			if len(user.Emails) != 1 {
				t.Errorf("should only preload one emails for user3 when with condition")
			}
		} else if len(user.Emails) != 0 {
			t.Errorf("should not preload any emails for other users when with condition (%d loaded)", len(user.Emails))
		} else if user.Emails == nil {
			t.Errorf("should return an empty slice to indicate zero results")
		}
	}
}

func TestNestedPreload1(t *testing.T) {
	//t.Log("79) TestNestedPreload1")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1   Level1
			Level3ID uint
		}
		Level3 struct {
			ID     uint
			Name   string
			Level2 Level2
		}
	)
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{Level2: Level2{Level1: Level1{Value: "value"}}}
	if err := TestDB.Create(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2").Preload("Level2.Level1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	if err := TestDB.Preload("Level2").Preload("Level2.Level1").Find(&got, "name = ?", "not_found").Error; err != gorm.ErrRecordNotFound {
		t.Error(err)
	}
}

func TestNestedPreload2(t *testing.T) {
	//t.Log("80) TestNestedPreload2")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1s  []*Level1
			Level3ID uint
		}
		Level3 struct {
			ID      uint
			Name    string
			Level2s []Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Level2s: []Level2{
			{
				Level1s: []*Level1{
					{Value: "value1"},
					{Value: "value2"},
				},
			},
			{
				Level1s: []*Level1{
					{Value: "value3"},
				},
			},
		},
	}
	if err := TestDB.Create(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2s.Level1s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload3(t *testing.T) {
	//t.Log("81) TestNestedPreload3")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1   Level1
			Level3ID uint
		}
		Level3 struct {
			Name    string
			ID      uint
			Level2s []Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Level2s: []Level2{
			{Level1: Level1{Value: "value1"}},
			{Level1: Level1{Value: "value2"}},
		},
	}
	if err := TestDB.Create(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2s.Level1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload4(t *testing.T) {
	//t.Log("82) TestNestedPreload4")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1s  []Level1
			Level3ID uint
		}
		Level3 struct {
			ID     uint
			Name   string
			Level2 Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Level2: Level2{
			Level1s: []Level1{
				{Value: "value1"},
				{Value: "value2"},
			},
		},
	}
	if err := TestDB.Create(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

// Slice: []Level3
func TestNestedPreload5(t *testing.T) {
	//t.Log("86) TestNestedPreload5")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1   Level1
			Level3ID uint
		}
		Level3 struct {
			ID     uint
			Name   string
			Level2 Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := make([]Level3, 2)
	want[0] = Level3{Level2: Level2{Level1: Level1{Value: "value"}}}
	if err := TestDB.Create(&want[0]).Error; err != nil {
		t.Error(err)
	}
	want[1] = Level3{Level2: Level2{Level1: Level1{Value: "value2"}}}
	if err := TestDB.Create(&want[1]).Error; err != nil {
		t.Error(err)
	}

	var got []Level3
	if err := TestDB.Preload("Level2").Preload("Level2.Level1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload6(t *testing.T) {
	//t.Log("87) TestNestedPreload6")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1s  []Level1
			Level3ID uint
		}
		Level3 struct {
			ID      uint
			Name    string
			Level2s []Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := make([]Level3, 2)
	want[0] = Level3{
		Level2s: []Level2{
			{
				Level1s: []Level1{
					{Value: "value1"},
					{Value: "value2"},
				},
			},
			{
				Level1s: []Level1{
					{Value: "value3"},
				},
			},
		},
	}
	if err := TestDB.Create(&want[0]).Error; err != nil {
		t.Error(err)
	}

	want[1] = Level3{
		Level2s: []Level2{
			{
				Level1s: []Level1{
					{Value: "value3"},
					{Value: "value4"},
				},
			},
			{
				Level1s: []Level1{
					{Value: "value5"},
				},
			},
		},
	}
	if err := TestDB.Create(&want[1]).Error; err != nil {
		t.Error(err)
	}

	var got []Level3
	if err := TestDB.Preload("Level2s.Level1s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload7(t *testing.T) {
	//t.Log("88) TestNestedPreload7")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1   Level1
			Level3ID uint
		}
		Level3 struct {
			ID      uint
			Name    string
			Level2s []Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := make([]Level3, 2)
	want[0] = Level3{
		Level2s: []Level2{
			{Level1: Level1{Value: "value1"}},
			{Level1: Level1{Value: "value2"}},
		},
	}
	if err := TestDB.Create(&want[0]).Error; err != nil {
		t.Error(err)
	}

	want[1] = Level3{
		Level2s: []Level2{
			{Level1: Level1{Value: "value3"}},
			{Level1: Level1{Value: "value4"}},
		},
	}
	if err := TestDB.Create(&want[1]).Error; err != nil {
		t.Error(err)
	}

	var got []Level3
	if err := TestDB.Preload("Level2s.Level1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload8(t *testing.T) {
	//t.Log("89) TestNestedPreload8")
	type (
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
		}
		Level2 struct {
			ID       uint
			Level1s  []Level1
			Level3ID uint
		}
		Level3 struct {
			ID     uint
			Name   string
			Level2 Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := make([]Level3, 2)
	want[0] = Level3{
		Level2: Level2{
			Level1s: []Level1{
				{Value: "value1"},
				{Value: "value2"},
			},
		},
	}
	if err := TestDB.Create(&want[0]).Error; err != nil {
		t.Error(err)
	}
	want[1] = Level3{
		Level2: Level2{
			Level1s: []Level1{
				{Value: "value3"},
				{Value: "value4"},
			},
		},
	}
	if err := TestDB.Create(&want[1]).Error; err != nil {
		t.Error(err)
	}

	var got []Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload9(t *testing.T) {
	//t.Log("90) TestNestedPreload9")
	type (
		Level0 struct {
			ID       uint
			Value    string
			Level1ID uint
		}
		Level1 struct {
			ID         uint
			Value      string
			Level2ID   uint
			Level2_1ID uint
			Level0s    []Level0
		}
		Level2 struct {
			ID       uint
			Level1s  []Level1
			Level3ID uint
		}
		Level2_1 struct {
			ID       uint
			Level1s  []Level1
			Level3ID uint
		}
		Level3 struct {
			ID       uint
			Name     string
			Level2   Level2
			Level2_1 Level2_1
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level2_1{})
	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level0{})
	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}, &Level2_1{}, &Level0{}).Error; err != nil {
		t.Error(err)
	}

	want := make([]Level3, 2)
	want[0] = Level3{
		Level2: Level2{
			Level1s: []Level1{
				{Value: "value1"},
				{Value: "value2"},
			},
		},
		Level2_1: Level2_1{
			Level1s: []Level1{
				{
					Value:   "value1-1",
					Level0s: []Level0{{Value: "Level0-1"}},
				},
				{
					Value:   "value2-2",
					Level0s: []Level0{{Value: "Level0-2"}},
				},
			},
		},
	}
	if err := TestDB.Create(&want[0]).Error; err != nil {
		t.Error(err)
	}
	want[1] = Level3{
		Level2: Level2{
			Level1s: []Level1{
				{Value: "value3"},
				{Value: "value4"},
			},
		},
		Level2_1: Level2_1{
			Level1s: []Level1{
				{
					Value:   "value3-3",
					Level0s: []Level0{},
				},
				{
					Value:   "value4-4",
					Level0s: []Level0{},
				},
			},
		},
	}
	if err := TestDB.Create(&want[1]).Error; err != nil {
		t.Error(err)
	}

	var got []Level3
	if err := TestDB.Preload("Level2").Preload("Level2.Level1s").Preload("Level2_1").Preload("Level2_1.Level1s").Preload("Level2_1.Level1s.Level0s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload10(t *testing.T) {
	//t.Log("91) TestNestedPreload10")
	TestDB.DropTableIfExists(&LevelA3{})
	TestDB.DropTableIfExists(&LevelA2{})
	TestDB.DropTableIfExists(&LevelA1{})

	if err := TestDB.AutoMigrate(&LevelA1{}, &LevelA2{}, &LevelA3{}).Error; err != nil {
		t.Error(err)
	}

	levelA1 := &LevelA1{Value: "foo"}
	if err := TestDB.Save(levelA1).Error; err != nil {
		t.Error(err)
	}

	want := []*LevelA2{
		{
			Value: "bar",
			LevelA3s: []*LevelA3{
				{
					Value:   "qux",
					LevelA1: levelA1,
				},
			},
		},
		{
			Value:    "bar 2",
			LevelA3s: []*LevelA3{},
		},
	}
	for _, levelA2 := range want {
		if err := TestDB.Save(levelA2).Error; err != nil {
			t.Error(err)
		}
	}

	var got []*LevelA2
	if err := TestDB.Preload("LevelA3s.LevelA1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload11(t *testing.T) {
	//t.Log("92) TestNestedPreload11")
	TestDB.DropTableIfExists(&LevelB2{})
	TestDB.DropTableIfExists(&LevelB3{})
	TestDB.DropTableIfExists(&LevelB1{})
	if err := TestDB.AutoMigrate(&LevelB1{}, &LevelB2{}, &LevelB3{}).Error; err != nil {
		t.Error(err)
	}

	levelB1 := &LevelB1{Value: "foo"}
	if err := TestDB.Create(levelB1).Error; err != nil {
		t.Error(err)
	}

	levelB3 := &LevelB3{
		Value:     "bar",
		LevelB1ID: sql.NullInt64{Valid: true, Int64: int64(levelB1.ID)},
	}
	if err := TestDB.Create(levelB3).Error; err != nil {
		t.Error(err)
	}
	levelB1.LevelB3s = []*LevelB3{levelB3}

	want := []*LevelB1{levelB1}
	var got []*LevelB1
	if err := TestDB.Preload("LevelB3s.LevelB2s").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestNestedPreload12(t *testing.T) {
	//t.Log("93) TestNestedPreload12")
	TestDB.DropTableIfExists(&LevelC2{})
	TestDB.DropTableIfExists(&LevelC3{})
	TestDB.DropTableIfExists(&LevelC1{})
	if err := TestDB.AutoMigrate(&LevelC1{}, &LevelC2{}, &LevelC3{}).Error; err != nil {
		t.Error(err)
	}

	level2 := LevelC2{
		Value: "c2",
		LevelC1: LevelC1{
			Value: "c1",
		},
	}
	TestDB.Create(&level2)

	want := []LevelC3{
		{
			Value:   "c3-1",
			LevelC2: level2,
		}, {
			Value:   "c3-2",
			LevelC2: level2,
		},
	}

	for i := range want {
		if err := TestDB.Create(&want[i]).Error; err != nil {
			t.Error(err)
		}
	}

	var got []LevelC3
	if err := TestDB.Preload("LevelC2").Preload("LevelC2.LevelC1").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestManyToManyPreloadWithMultiPrimaryKeys(t *testing.T) {
	//t.Log("94) TestManyToManyPreloadWithMultiPrimaryKeys")
	if dialect := os.Getenv("GORM_DIALECT"); dialect == "" || dialect == "sqlite" {
		return
	}

	type (
		Level1 struct {
			ID           uint   `gorm:"primary_key;"`
			LanguageCode string `gorm:"primary_key"`
			Value        string
		}
		Level2 struct {
			ID           uint   `gorm:"primary_key;"`
			LanguageCode string `gorm:"primary_key"`
			Value        string
			Level1s      []Level1 `gorm:"many2many:levels;"`
		}
	)

	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists("levels")

	if err := TestDB.AutoMigrate(&Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level2{Value: "Bob", LanguageCode: "ru", Level1s: []Level1{
		{Value: "ru", LanguageCode: "ru"},
		{Value: "en", LanguageCode: "en"},
	}}
	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	want2 := Level2{Value: "Tom", LanguageCode: "zh", Level1s: []Level1{
		{Value: "zh", LanguageCode: "zh"},
		{Value: "de", LanguageCode: "de"},
	}}
	if err := TestDB.Save(&want2).Error; err != nil {
		t.Error(err)
	}

	var got Level2
	if err := TestDB.Preload("Level1s").Find(&got, "value = ?", "Bob").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	var got2 Level2
	if err := TestDB.Preload("Level1s").Find(&got2, "value = ?", "Tom").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("got %s; want %s", toJSONString(got2), toJSONString(want2))
	}

	var got3 []Level2
	if err := TestDB.Preload("Level1s").Find(&got3, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got3, []Level2{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got3), toJSONString([]Level2{got, got2}))
	}

	var got4 []Level2
	if err := TestDB.Preload("Level1s", "value IN (?)", []string{"zh", "ru"}).Find(&got4, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	var ruLevel1 Level1
	var zhLevel1 Level1
	TestDB.First(&ruLevel1, "value = ?", "ru")
	TestDB.First(&zhLevel1, "value = ?", "zh")

	got.Level1s = []Level1{ruLevel1}
	got2.Level1s = []Level1{zhLevel1}
	if !reflect.DeepEqual(got4, []Level2{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got4), toJSONString([]Level2{got, got2}))
	}

	if err := TestDB.Preload("Level1s").Find(&got4, "value IN (?)", []string{"non-existing"}).Error; err != nil {
		t.Error(err)
	}
}

func TestManyToManyPreloadForNestedPointer(t *testing.T) {
	//t.Log("95) TestManyToManyPreloadForNestedPointer")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []*Level1 `gorm:"many2many:levels;"`
		}
		Level3 struct {
			ID       uint
			Value    string
			Level2ID sql.NullInt64
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists("levels")

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Value: "Bob",
		Level2: &Level2{
			Value: "Foo",
			Level1s: []*Level1{
				{Value: "ru"},
				{Value: "en"},
			},
		},
	}
	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	want2 := Level3{
		Value: "Tom",
		Level2: &Level2{
			Value: "Bar",
			Level1s: []*Level1{
				{Value: "zh"},
				{Value: "de"},
			},
		},
	}
	if err := TestDB.Save(&want2).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got, "value = ?", "Bob").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	var got2 Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got2, "value = ?", "Tom").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("got %s; want %s", toJSONString(got2), toJSONString(want2))
	}

	var got3 []Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got3, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got3, []Level3{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got3), toJSONString([]Level3{got, got2}))
	}

	var got4 []Level3
	if err := TestDB.Preload("Level2.Level1s", "value IN (?)", []string{"zh", "ru"}).Find(&got4, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	var got5 Level3
	TestDB.Preload("Level2.Level1s").Find(&got5, "value = ?", "bogus")

	var ruLevel1 Level1
	var zhLevel1 Level1
	TestDB.First(&ruLevel1, "value = ?", "ru")
	TestDB.First(&zhLevel1, "value = ?", "zh")

	got.Level2.Level1s = []*Level1{&ruLevel1}
	got2.Level2.Level1s = []*Level1{&zhLevel1}
	if !reflect.DeepEqual(got4, []Level3{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got4), toJSONString([]Level3{got, got2}))
	}
}

func TestNestedManyToManyPreload(t *testing.T) {
	//t.Log("96) TestNestedManyToManyPreload")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []*Level1 `gorm:"many2many:level1_level2;"`
		}
		Level3 struct {
			ID      uint
			Value   string
			Level2s []Level2 `gorm:"many2many:level2_level3;"`
		}
	)

	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists("level1_level2")
	TestDB.DropTableIfExists("level2_level3")

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Value: "Level3",
		Level2s: []Level2{
			{
				Value: "Bob",
				Level1s: []*Level1{
					{Value: "ru"},
					{Value: "en"},
				},
			}, {
				Value: "Tom",
				Level1s: []*Level1{
					{Value: "zh"},
					{Value: "de"},
				},
			},
		},
	}

	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2s").Preload("Level2s.Level1s").Find(&got, "value = ?", "Level3").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	if err := TestDB.Preload("Level2s.Level1s").Find(&got, "value = ?", "not_found").Error; err != gorm.ErrRecordNotFound {
		t.Error(err)
	}
}

func TestNestedManyToManyPreload2(t *testing.T) {
	//t.Log("97) TestNestedManyToManyPreload TWO")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []*Level1 `gorm:"many2many:level1_level2;"`
		}
		Level3 struct {
			ID       uint
			Value    string
			Level2ID sql.NullInt64
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists("level1_level2")

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level3{
		Value: "Level3",
		Level2: &Level2{
			Value: "Bob",
			Level1s: []*Level1{
				{Value: "ru"},
				{Value: "en"},
			},
		},
	}

	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	var got Level3
	if err := TestDB.Preload("Level2.Level1s").Find(&got, "value = ?", "Level3").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	if err := TestDB.Preload("Level2.Level1s").Find(&got, "value = ?", "not_found").Error; err != gorm.ErrRecordNotFound {
		t.Error(err)
	}
}

func TestNestedManyToManyPreload3(t *testing.T) {
	//t.Log("98) TestNestedManyToManyPreload3")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []*Level1 `gorm:"many2many:level1_level2;"`
		}
		Level3 struct {
			ID       uint
			Value    string
			Level2ID sql.NullInt64
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists("level1_level2")

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	level1Zh := &Level1{Value: "zh"}
	level1Ru := &Level1{Value: "ru"}
	level1En := &Level1{Value: "en"}

	level21 := &Level2{
		Value:   "Level2-1",
		Level1s: []*Level1{level1Zh, level1Ru},
	}

	level22 := &Level2{
		Value:   "Level2-2",
		Level1s: []*Level1{level1Zh, level1En},
	}

	wants := []*Level3{
		{
			Value:  "Level3-1",
			Level2: level21,
		},
		{
			Value:  "Level3-2",
			Level2: level22,
		},
		{
			Value:  "Level3-3",
			Level2: level21,
		},
	}

	for _, want := range wants {
		if err := TestDB.Save(&want).Error; err != nil {
			t.Error(err)
		}
	}

	var gots []*Level3
	if err := TestDB.Preload("Level2.Level1s", func(db *gorm.DBCon) *gorm.DBCon {
		return db.Order("level1.id ASC")
	}).Find(&gots).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(gots, wants) {
		t.Errorf("got %s; want %s", toJSONString(gots), toJSONString(wants))
	}
}

func TestNestedManyToManyPreload3ForStruct(t *testing.T) {
	//t.Log("99) TestNestedManyToManyPreload3ForStruct")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []Level1 `gorm:"many2many:level1_level2;"`
		}
		Level3 struct {
			ID       uint
			Value    string
			Level2ID sql.NullInt64
			Level2   Level2
		}
	)

	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists("level1_level2")

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	level1Zh := Level1{Value: "zh"}
	level1Ru := Level1{Value: "ru"}
	level1En := Level1{Value: "en"}

	level21 := Level2{
		Value:   "Level2-1",
		Level1s: []Level1{level1Zh, level1Ru},
	}

	level22 := Level2{
		Value:   "Level2-2",
		Level1s: []Level1{level1Zh, level1En},
	}

	wants := []*Level3{
		{
			Value:  "Level3-1",
			Level2: level21,
		},
		{
			Value:  "Level3-2",
			Level2: level22,
		},
		{
			Value:  "Level3-3",
			Level2: level21,
		},
	}

	for _, want := range wants {
		if err := TestDB.Save(&want).Error; err != nil {
			t.Error(err)
		}
	}

	var gots []*Level3
	if err := TestDB.Preload("Level2.Level1s", func(db *gorm.DBCon) *gorm.DBCon {
		return db.Order("level1.id ASC")
	}).Find(&gots).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(gots, wants) {
		t.Errorf("got %s; want %s", toJSONString(gots), toJSONString(wants))
	}
}

func TestNestedManyToManyPreload4(t *testing.T) {
	//t.Log("100) TestNestedManyToManyPreload4")
	type (
		Level4 struct {
			ID       uint
			Value    string
			Level3ID uint
		}
		Level3 struct {
			ID      uint
			Value   string
			Level4s []*Level4
		}
		Level2 struct {
			ID      uint
			Value   string
			Level3s []*Level3 `gorm:"many2many:level2_level3;"`
		}
		Level1 struct {
			ID      uint
			Value   string
			Level2s []*Level2 `gorm:"many2many:level1_level2;"`
		}
	)

	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level4{})
	TestDB.DropTableIfExists("level1_level2")
	TestDB.DropTableIfExists("level2_level3")

	dummy := Level1{
		Value: "Level1",
		Level2s: []*Level2{{
			Value: "Level2",
			Level3s: []*Level3{{
				Value: "Level3",
				Level4s: []*Level4{{
					Value: "Level4",
				}},
			}},
		}},
	}

	if err := TestDB.AutoMigrate(&Level4{}, &Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	if err := TestDB.Save(&dummy).Error; err != nil {
		t.Error(err)
	}

	var level1 Level1
	if err := TestDB.Preload("Level2s").Preload("Level2s.Level3s").Preload("Level2s.Level3s.Level4s").First(&level1).Error; err != nil {
		t.Error(err)
	}
}

func TestManyToManyPreloadForPointer(t *testing.T) {
	//t.Log("101) TestManyToManyPreloadForPointer")
	type (
		Level1 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID      uint
			Value   string
			Level1s []*Level1 `gorm:"many2many:levels;"`
		}
	)

	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})
	TestDB.DropTableIfExists("levels")

	if err := TestDB.AutoMigrate(&Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level2{Value: "Bob", Level1s: []*Level1{
		{Value: "ru"},
		{Value: "en"},
	}}
	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	want2 := Level2{Value: "Tom", Level1s: []*Level1{
		{Value: "zh"},
		{Value: "de"},
	}}
	if err := TestDB.Save(&want2).Error; err != nil {
		t.Error(err)
	}

	var got Level2
	if err := TestDB.Preload("Level1s").Find(&got, "value = ?", "Bob").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}

	var got2 Level2
	if err := TestDB.Preload("Level1s").Find(&got2, "value = ?", "Tom").Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got2, want2) {
		t.Errorf("got %s; want %s", toJSONString(got2), toJSONString(want2))
	}

	var got3 []Level2
	if err := TestDB.Preload("Level1s").Find(&got3, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got3, []Level2{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got3), toJSONString([]Level2{got, got2}))
	}

	var got4 []Level2
	if err := TestDB.Preload("Level1s", "value IN (?)", []string{"zh", "ru"}).Find(&got4, "value IN (?)", []string{"Bob", "Tom"}).Error; err != nil {
		t.Error(err)
	}

	var got5 Level2
	TestDB.Preload("Level1s").First(&got5, "value = ?", "bogus")

	var ruLevel1 Level1
	var zhLevel1 Level1
	TestDB.First(&ruLevel1, "value = ?", "ru")
	TestDB.First(&zhLevel1, "value = ?", "zh")

	got.Level1s = []*Level1{&ruLevel1}
	got2.Level1s = []*Level1{&zhLevel1}
	if !reflect.DeepEqual(got4, []Level2{got, got2}) {
		t.Errorf("got %s; want %s", toJSONString(got4), toJSONString([]Level2{got, got2}))
	}
}

func TestNilPointerSlice(t *testing.T) {
	//t.Log("102) TestNilPointerSlice")
	type (
		Level3 struct {
			ID    uint
			Value string
		}
		Level2 struct {
			ID       uint
			Value    string
			Level3ID uint
			Level3   *Level3
		}
		Level1 struct {
			ID       uint
			Value    string
			Level2ID uint
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(&Level3{})
	TestDB.DropTableIfExists(&Level2{})
	TestDB.DropTableIfExists(&Level1{})

	if err := TestDB.AutoMigrate(&Level3{}, &Level2{}, &Level1{}).Error; err != nil {
		t.Error(err)
	}

	want := Level1{
		Value: "Bob",
		Level2: &Level2{
			Value: "en",
			Level3: &Level3{
				Value: "native",
			},
		},
	}
	if err := TestDB.Save(&want).Error; err != nil {
		t.Error(err)
	}

	want2 := Level1{
		Value:  "Tom",
		Level2: nil,
	}
	if err := TestDB.Save(&want2).Error; err != nil {
		t.Error(err)
	}

	var got []Level1
	if err := TestDB.Preload("Level2").Preload("Level2.Level3").Find(&got).Error; err != nil {
		t.Error(err)
	}

	if len(got) != 2 {
		t.Errorf("got %v items, expected 2", len(got))
	}

	if !reflect.DeepEqual(got[0], want) && !reflect.DeepEqual(got[1], want) {
		t.Errorf("got %s; want array containing %s", toJSONString(got), toJSONString(want))
	}

	if !reflect.DeepEqual(got[0], want2) && !reflect.DeepEqual(got[1], want2) {
		t.Errorf("got %s; want array containing %s", toJSONString(got), toJSONString(want2))
	}
}

func TestNilPointerSlice2(t *testing.T) {
	//t.Log("103) TestNilPointerSlice2")
	type (
		Level4 struct {
			ID uint
		}
		Level3 struct {
			ID       uint
			Level4ID sql.NullInt64 `sql:"index"`
			Level4   *Level4
		}
		Level2 struct {
			ID      uint
			Level3s []*Level3 `gorm:"many2many:level2_level3s"`
		}
		Level1 struct {
			ID       uint
			Level2ID sql.NullInt64 `sql:"index"`
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(new(Level4))
	TestDB.DropTableIfExists(new(Level3))
	TestDB.DropTableIfExists(new(Level2))
	TestDB.DropTableIfExists(new(Level1))

	if err := TestDB.AutoMigrate(new(Level4), new(Level3), new(Level2), new(Level1)).Error; err != nil {
		t.Error(err)
	}

	want := new(Level1)
	if err := TestDB.Save(want).Error; err != nil {
		t.Error(err)
	}

	got := new(Level1)
	err := TestDB.Preload("Level2.Level3s.Level4").Last(&got).Error
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}

func TestPrefixedPreloadDuplication(t *testing.T) {
	//t.Log("104) TestPrefixedPreloadDuplication")
	type (
		Level4 struct {
			ID       uint
			Name     string
			Level3ID uint
		}
		Level3 struct {
			ID      uint
			Name    string
			Level4s []*Level4
		}
		Level2 struct {
			ID       uint
			Name     string
			Level3ID sql.NullInt64 `sql:"index"`
			Level3   *Level3
		}
		Level1 struct {
			ID       uint
			Name     string
			Level2ID sql.NullInt64 `sql:"index"`
			Level2   *Level2
		}
	)

	TestDB.DropTableIfExists(new(Level3))
	TestDB.DropTableIfExists(new(Level4))
	TestDB.DropTableIfExists(new(Level2))
	TestDB.DropTableIfExists(new(Level1))

	if err := TestDB.AutoMigrate(new(Level3), new(Level4), new(Level2), new(Level1)).Error; err != nil {
		t.Error(err)
	}

	lvl := &Level3{}
	if err := TestDB.Save(lvl).Error; err != nil {
		t.Error(err)
	}

	sublvl1 := &Level4{Level3ID: lvl.ID}
	if err := TestDB.Save(sublvl1).Error; err != nil {
		t.Error(err)
	}
	sublvl2 := &Level4{Level3ID: lvl.ID}
	if err := TestDB.Save(sublvl2).Error; err != nil {
		t.Error(err)
	}

	lvl.Level4s = []*Level4{sublvl1, sublvl2}

	want1 := Level1{
		Level2: &Level2{
			Level3: lvl,
		},
	}
	if err := TestDB.Save(&want1).Error; err != nil {
		t.Error(err)
	}

	want2 := Level1{
		Level2: &Level2{
			Level3: lvl,
		},
	}
	if err := TestDB.Save(&want2).Error; err != nil {
		t.Error(err)
	}

	want := []Level1{want1, want2}

	var got []Level1
	err := TestDB.Preload("Level2.Level3.Level4s").Find(&got).Error
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s; want %s", toJSONString(got), toJSONString(want))
	}
}
