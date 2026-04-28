// Package main demonstrates runtime AI endpoints and small HTTP helpers.
package main

import (
	"net/http"

	igo "github.com/leebo/igo"
)

type UserPath struct {
	ID int64 `json:"id" validate:"required|gte:1"`
}

type UserResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := igo.Simple()
	app.RegisterSchema(UserResponse{})

	app.Get("/users/:id", func(c *igo.Context) {
		params, ok := igo.BindPathAndValidate[UserPath](c)
		if !ok {
			return
		}
		c.Success(UserResponse{ID: params.ID, Name: "Ada"})
	})

	app.Get("/cookie", func(c *igo.Context) {
		value, err := c.Cookie("session")
		if err != nil {
			c.BadRequestWrap(err, "missing session cookie")
			return
		}
		c.SetCookie("last_session", value, 3600, "", "", false, true)
		c.Success(igo.H{"session": value})
	})

	app.Get("/login", func(c *igo.Context) {
		c.Redirect(http.StatusFound, "/users/1")
	})

	app.Static("/assets", "./examples/ai_runtime/public")
	app.RegisterAIRoutes()
	app.PrintRoutes()
	app.Run(":8080")
}
