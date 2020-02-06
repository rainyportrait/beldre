package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"github.com/rainyportrait/beldre/conf"
)

// Possible errors
var (
	ErrUsernameLength            = errors.New("username has to be between 4 and 30 characters long")
	ErrUsernameInvalidCharacters = errors.New("username may only contain numbers and letters")
	ErrUsernameInUse             = errors.New("username already in use")
	ErrNoUser                    = errors.New("no user with that name exists")
	ErrPasswordLength            = errors.New("password has to be at least 8 characters long")
	ErrPasswordsDontMatch        = errors.New("given and stored password do not match")
)

// RegisterUser - POST /user/register
func RegisterUser(router *gin.RouterGroup, conf *conf.Config) {
	router.POST("/user/register", func(c *gin.Context) {
		name := strings.ToLower(c.PostForm("name"))

		if err := ValidUsername([]rune(name)); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		hash, err := HashPassword(c.PostForm("password"))
		if err == ErrPasswordLength {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		} else if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		sqlStr, _ := conf.GetTemplateString("insert_user", nil)
		res, err := conf.DB.Exec(sqlStr, name, hash)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}
		if ra, _ := res.RowsAffected(); ra == 0 {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": ErrUsernameInUse.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"msg": "We did it reddit!",
		})
	})
}

// LoginUser - POST /api/v1/user/login
func LoginUser(router *gin.RouterGroup, conf *conf.Config) {
	router.POST("/user/login", func(c *gin.Context) {
		name := strings.ToLower(c.PostForm("name"))

		sqlStr, err := conf.GetTemplateString("select_user", struct{ Password bool }{true})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		u := User{}
		if err := conf.DB.Get(&u, sqlStr, name); err == sql.ErrNoRows {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": ErrNoUser.Error(),
			})
			return
		} else if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		if err := u.ComparePassword(c.PostForm("password")); err == ErrPasswordLength {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		} else if err == ErrPasswordsDontMatch {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		} else if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"msg": "We did it reddit!",
		})
	})
}

// User - User information
// Password is expected to be empty for everything but login
type User struct {
	ID int `json:"id"`

	Name     string `json:"name"`
	Password []byte `json:"-"`
	Level    string `json:"level"`

	CreatedAt string  `json:"createdAt" db:"created_at"`
	UpdatedAt *string `json:"updatedAt,omitempty" db:"updated_at"`
}

// ComparePassword - Compares argument with stored hash
func (u *User) ComparePassword(password string) error {
	if err := ValidPassword(password); err != nil {
		return err
	}

	err := bcrypt.CompareHashAndPassword(u.Password, []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return ErrPasswordsDontMatch
	}

	return err
}

// HashPassword - Checks and hashes password
func HashPassword(password string) ([]byte, error) {
	if err := ValidPassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	return hash, nil
}

// ValidPassword - Test if password is valid
func ValidPassword(password string) error {
	if utf8.RuneCount([]byte(password)) < 8 {
		return ErrPasswordLength
	}

	return nil
}

// ValidUsername - Tests if username is valid
func ValidUsername(name []rune) error {
	// Good enough as we don't allow utf8 chracters in names
	l := len(name)
	if l < 4 || l > 30 {
		return ErrUsernameLength
	}

	for _, r := range name {
		if !(('a' <= r && r <= 'z') || unicode.IsDigit(r)) {
			return ErrUsernameInvalidCharacters
		}
	}

	return nil
}
