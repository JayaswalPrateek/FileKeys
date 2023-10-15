package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type File struct {
	gorm.Model
	pdfBlob []byte
	pdfHash string
	ofcBlob []byte
	ofcHash string
}

func loadDB() *gorm.DB {
	log.Info("connecting to database...")
	db, err := gorm.Open(sqlite.Open("main.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("database connection failed")
	}
	err = db.AutoMigrate(&File{})
	if err != nil {
		log.Fatal("db schema migration failed")
	}
	log.Info("database connected, preparing server...")
	return db
}

func loadHTML() { // todo: windows support pending
	log.Info("opening main.html")

	os := runtime.GOOS
	if os == "linux" {
		cmd := exec.Command("xdg-open", "main.html")
		if err := cmd.Run(); err != nil {
			log.Error(err)
		}
	} else {
		log.Fatal("Unsupported Platform, aborting...")
	}

	log.Info("main.html has been opened")
}

func loadRouter(db *gorm.DB) (*multipart.FileHeader, string, *gorm.DB) {
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20
	var emailID string
	var file *multipart.FileHeader
	router.POST("/upload", func(c *gin.Context) {
		emailID = c.PostForm("mailID")
		file, _ = c.FormFile("uploadedFile")
	})
	return file, emailID, db
}
func pipeline(file *multipart.FileHeader, emailID string, db *gorm.DB) {
	uploadedFile, _ := file.Open()
	defer uploadedFile.Close()
	localFile, _ := os.Create("./tmp." + inferFileFmt(uploadedFile))
	defer localFile.Close()
	io.Copy(localFile, uploadedFile)

	hashOfLocalFile := computeSHA256Hash("./tmp." + inferFileFmt(uploadedFile))
	// check if converted file cached in db if so then mail it
	targetFmt := inferTargetFmt(uploadedFile)
	// else use npm cli tool to convert, cache and mail

	/*
		APIs to use
		2. Email converted file using https://www.mailjet.com/products/email-api/ or
		https://sendgrid.com/solutions/email-api/   or
		https://www.mailersend.com/features/email-api
		file handling https://github.com/spf13/afero
	*/

}
func inferTargetFmt(uploadedFile multipart.File) string {
	buf, _ := io.ReadAll(uploadedFile)
	if filetype.IsDocument(buf) {
		return "docx"
	}
	return "pdf"
}
func inferFileFmt(uploadedFile multipart.File) string {
	buf, _ := io.ReadAll(uploadedFile)
	kind, _ := filetype.Match(buf)
	return kind.Extension

}
func computeSHA256Hash(filePath string) (string, error) {
	file, _ := os.Open(filePath)
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// Convert the hash to a hexadecimal string
	hashSum := hash.Sum(nil)
	hashString := hex.EncodeToString(hashSum)

	return hashString, nil
}

func main() {
	db := loadDB()
	loadHTML()
	pipeline(loadRouter(db))
}
