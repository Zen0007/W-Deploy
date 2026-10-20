package service

import "fmt"

// resolveIdentity menentukan user & dbName final yang dipakai baik untuk
// env container maupun connection URI, sehingga keduanya tidak pernah beda.
func resolveIdentity(engine DBEngine, req DBProvisionRequest) (user, dbName string, err error) {
	dbName = req.DBName
	if dbName == "" {
		dbName = req.AppSlug
	}

	user = req.Username
	if user == "" {
		if engine == DBMongo {
			user = "admin"
		} else {
			user = req.AppSlug
		}
	}

	if user == "" || dbName == "" {
		return "", "", fmt.Errorf("user/database tidak boleh kosong")
	}
	return user, dbName, nil
}
