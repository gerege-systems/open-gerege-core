// Gerege Template Version 27.0
// Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

// Package gspace нь "Gerege Space" — апп-ын өөрийн SFTP хадгалалтын client.
// Хэрэглэгч бүр өөрийн хавтастай (basePath/users/<userID>/) бөгөөд usecase давхаргад
// квот (default 2MB) шалгагдана. OAuth-гүй — апп нэг SFTP данс (нууц env-д) ашиглаж,
// файлыг хэрэглэгч-тус-бүрийн замаар тусгаарлана.
package gspace

import (
	"bytes"
	"errors"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// ErrNotConfigured — SFTP host/user/password тохируулаагүй.
var ErrNotConfigured = errors.New("gspace: SFTP storage not configured")

// FileInfo — хэрэглэгчийн Gerege Space дахь нэг файл.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// Config — SFTP холболтын тохиргоо.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	BasePath string // home-оос харьцангуй үндсэн хавтас (ж: "gerege-space")
}

// Client нь SFTP хадгалалтын client. Дуудлага бүрд шинэ холболт үүсгэнэ (файлын
// үйлдэл ховор тул pool шаардлагагүй).
type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if strings.TrimSpace(cfg.BasePath) == "" {
		cfg.BasePath = "gerege-space"
	}
	return &Client{cfg: cfg}
}

// Configured — SFTP тохируулагдсан эсэх.
func (c *Client) Configured() bool {
	return c.cfg.Host != "" && c.cfg.User != "" && c.cfg.Password != ""
}

func (c *Client) userDir(userID string) string {
	return path.Join(c.cfg.BasePath, "users", safeSegment(userID))
}

// withSFTP нь SSH+SFTP холболт үүсгэж, fn-д client дамжуулаад хаана.
func (c *Client) withSFTP(fn func(*sftp.Client) error) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	sshCfg := &ssh.ClientConfig{
		User:            c.cfg.User,
		Auth:            []ssh.AuthMethod{ssh.Password(c.cfg.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // өөрийн storage host; ирээдүйд host key pin хийх
		Timeout:         12 * time.Second,
	}
	addr := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	conn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	sc, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer func() { _ = sc.Close() }()
	return fn(sc)
}

// List нь хэрэглэгчийн файлуудыг буцаана (хавтас байхгүй бол хоосон).
func (c *Client) List(userID string) ([]FileInfo, error) {
	var out []FileInfo
	err := c.withSFTP(func(sc *sftp.Client) error {
		dir := c.userDir(userID)
		entries, rdErr := sc.ReadDir(dir)
		if rdErr != nil {
			// Хавтас хараахан үүсээгүй бол хоосон жагсаалт (алдаа биш).
			return nil //nolint:nilerr
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			out = append(out, FileInfo{Name: e.Name(), Size: e.Size(), ModTime: e.ModTime()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Usage нь хэрэглэгчийн нийт эзэлхүүнийг (байт) буцаана.
func (c *Client) Usage(userID string) (int64, error) {
	files, err := c.List(userID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total, nil
}

// Upload нь файлыг хэрэглэгчийн хавтаст (байхгүй бол үүсгэж) бичнэ.
func (c *Client) Upload(userID, name string, data []byte) error {
	fn := safeSegment(name)
	if fn == "" {
		return errors.New("gspace: invalid file name")
	}
	return c.withSFTP(func(sc *sftp.Client) error {
		dir := c.userDir(userID)
		if err := sc.MkdirAll(dir); err != nil {
			return err
		}
		f, err := sc.Create(path.Join(dir, fn))
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(f, bytes.NewReader(data))
		return err
	})
}

// Download нь файлын агуулгыг буцаана.
func (c *Client) Download(userID, name string) ([]byte, error) {
	fn := safeSegment(name)
	if fn == "" {
		return nil, errors.New("gspace: invalid file name")
	}
	var data []byte
	err := c.withSFTP(func(sc *sftp.Client) error {
		f, oErr := sc.Open(path.Join(c.userDir(userID), fn))
		if oErr != nil {
			return oErr
		}
		defer func() { _ = f.Close() }()
		b, rErr := io.ReadAll(f)
		if rErr != nil {
			return rErr
		}
		data = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Delete нь файлыг устгана.
func (c *Client) Delete(userID, name string) error {
	fn := safeSegment(name)
	if fn == "" {
		return errors.New("gspace: invalid file name")
	}
	return c.withSFTP(func(sc *sftp.Client) error {
		return sc.Remove(path.Join(c.userDir(userID), fn))
	})
}

// safeSegment нь замын нэг сегментийг (файл/хэрэглэгч) аюулгүй болгоно — зөвхөн
// суурь нэр, ".."/"/"-гүй (path traversal-аас хамгаална).
func safeSegment(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "/")
	s = path.Base(s)
	if s == "." || s == ".." || s == "/" {
		return ""
	}
	return s
}
