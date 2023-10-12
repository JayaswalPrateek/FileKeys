/*
supported conversions:
pdf to office (costs four credits)
office to pdf (costs four credits)
mp4 to mp3 	  (costs one  credit)
*/
package main

import (
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type File struct {
	gorm.Model
	Filename  string
	Extension string
	Checksum  string
	Data      []byte
}

func loadDB() (db *gorm.DB) {
	log.Info("connecting to database...")
	db, err := gorm.Open(sqlite.Open("Files.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	err = db.AutoMigrate(&File{})
	if err != nil {
		log.Fatal("database migration failed")
	}
	log.Info("database connected, preparing server...")
	return
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

func loadRouter(db *gorm.DB) {
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20
	router.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{
				"error": "File not received",
			})
			return
		}
		toFormat := c.PostForm("to_format")
		emailID := c.PostForm("email_id")
		pipeline(file, toFormat, emailID, db)
		c.JSON(200, gin.H{
			"message": "File received and processed, check your mailbox",
		})
	})
}
func pipeline(file *multipart.FileHeader, targetFormat string, emailID string, db *gorm.DB) {
	/*
		APIs to use
		1. cloud convert API https://cloudconvert.com/api/v2#overview or https://www.convertapi.com/doc/go-library
		2. Email converted file using https://www.mailjet.com/products/email-api/ or
									  https://sendgrid.com/solutions/email-api/   or
									  https://www.mailersend.com/features/email-api
		file handling https://github.com/spf13/afero
	*/
	/*
		check inside db, with userID to get filenames, see if the filename exists
		if the filename exists see if the extension string exists, if so find its checksum
		find the associated file and mail it to the user

	*/
	/*
		then construct the file inside the current folder
		and use cloudconvert api cli interface to convert the original file to the target format
		after conversion, its checksum is calculated and stored in the database and then emailed to the user
	*/
	reconstructFileLocally(file)
}
func reconstructFileLocally(file *multipart.FileHeader) {
	uploadedFile, err := file.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer uploadedFile.Close()

	destinationPath := file.Filename
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		log.Fatal(err)
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, uploadedFile)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	db := loadDB()
	loadHTML()
	loadRouter(db)
}
