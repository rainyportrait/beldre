package conf

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os"
	"text/template"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/hashicorp/go-envparse"
	"github.com/jmoiron/sqlx"
)

// Config - Stores app configuration
type Config struct {
	DB            *sqlx.DB
	ConnectionURL string
	Secret        string
	ImagePath     string
	Templates     *template.Template
}

// ArgonParams - Configures Argon2
type ArgonParams struct {
	Memory       uint32
	Iterations   uint32
	Parrallelism uint8
	SaltLength   uint32
	KeyLength    uint32
}

// ConnectToDatabase - Connects to database using the ConnectionURL
func (c *Config) ConnectToDatabase() {
	u, err := url.Parse(c.ConnectionURL)
	if err != nil {
		log.Panic(err)
	}

	c.DB, err = sqlx.Open(
		u.Scheme,
		fmt.Sprintf("%s@%s%s", u.User, u.Host, u.Path),
	)
	if err != nil {
		log.Panic(err)
	}
}

// LoadEnv - Loads .env file
func (c *Config) LoadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		log.Panic(err)
	}
	defer f.Close()

	env, err := envparse.Parse(f)
	if err != nil {
		log.Panic(err)
	}

	var ok bool
	if c.ConnectionURL, ok = env["DATABASE_URL"]; !ok {
		log.Panic("DATABASE_URL missing from .env")
	}

	if c.ImagePath, ok = env["IMAGE_PATH"]; !ok {
		log.Panic("IMAGE_PATH missing from .env")
	}

	if c.Secret, ok = env["SECRET"]; !ok {
		log.Panic("SECRET missing from .env")
	}
}

// LoadTemplates - Loads SQL templates
func (c *Config) LoadTemplates() {
	files := []string{
		"select_posts",
		"select_tag_stats",
		"select_post",
		"select_post_tags",
		"select_user",
		"insert_user",
	}

	for i, v := range files {
		files[i] = fmt.Sprintf("db/templates/%s.tmpl.sql", v)
	}

	c.Templates = template.Must(template.ParseFiles(files...))
}

// GetTemplateString - Returns after executing template
func (c *Config) GetTemplateString(name string, data interface{}) (string, error) {
	buf := bytes.Buffer{}
	err := c.Templates.ExecuteTemplate(&buf, fmt.Sprintf("%s.tmpl.sql", name), &data)
	if err != nil {
		return "", err
	}

	return buf.String(), err
}
