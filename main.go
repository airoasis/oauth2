package main

import (
	"context"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-resty/resty/v2"
	"github.com/jackc/pgx/v4"
	"github.com/tidwall/gjson"
	pg "github.com/vgarvardt/go-oauth2-pg/v4"
	"github.com/vgarvardt/go-pg-adapter/pgx4adapter"
	"log"
	"net/http"
	"time"
)

func main() {
	pgxConn, _ := pgx.Connect(context.TODO(), "postgres://postgres:postgres@postgres:5432/postgres")

	manager := manage.NewDefaultManager()

	adapter := pgx4adapter.NewConn(pgxConn)
	tokenStore, _ := pg.NewTokenStore(adapter, pg.WithTokenStoreGCInterval(time.Minute))
	defer tokenStore.Close()

	// client pg store
	clientStore, _ := pg.NewClientStore(adapter)

	clientStore.Create(&models.Client{
		ID:     "222222",
		Secret: "22222222",
		Domain: "http://localhost:9096",
	})

	manager.MapTokenStorage(tokenStore)
	manager.MapClientStorage(clientStore)

	srv := server.NewServer(server.NewConfig(), manager)
	srv.SetAllowedGrantType(oauth2.PasswordCredentials)
	srv.SetAllowGetAccessRequest(true)

	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		log.Println("Internal Error:", err.Error())
		return
	})

	srv.SetResponseErrorHandler(func(re *errors.Response) {
		log.Println("Response Error:", re.Error.Error())
	})

	srv.SetPasswordAuthorizationHandler(func(username, password string) (userID string, err error) {
		client := resty.New()

		resp, err := client.R().
			SetBody(map[string]interface{}{
				"username": username,
				"password": password,
			}).
			Post("http://auth:8080/users/oauth")

		if err != nil {
			log.Println("ERROR sending the request")
			return
		}
		if resp.StatusCode() == 200 {
			userID = gjson.Get(resp.String(), "username").String()
		}
		return
	})

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleTokenRequest(w, r)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}