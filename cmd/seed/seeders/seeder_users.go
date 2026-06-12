// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

package seeders

import (
	"template/internal/constants"
	"template/internal/datasources/records"
	"template/pkg/helpers"
	"template/pkg/logger"
)

var pass string
var UserData []records.Users

func init() {
	var err error
	pass, err = helpers.GenerateHash("12345")
	if err != nil {
		logger.Panic(err.Error(), logger.Fields{constants.LoggerCategory: constants.LoggerCategorySeeder})
	}

	// Email/Password нь records.Users-д *string (eID хэрэглэгчид NULL байж
	// болох тул) — seed утгуудыг pointer болгож дамжуулна.
	patrickEmail := "patrick@gmail.com"
	johnEmail := "johndoe@gmail.com"
	UserData = []records.Users{
		{
			Username: "patrick star 7",
			Email:    &patrickEmail,
			Password: &pass,
			Active:   true,
			RoleId:   1,
		},
		{
			Username: "john doe",
			Email:    &johnEmail,
			Password: &pass,
			Active:   false,
			RoleId:   2,
		},
	}
}
