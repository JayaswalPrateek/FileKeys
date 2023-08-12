/*
supported conversions:
	pdf to office (costs four credits)
	office to pdf (costs four credits)
	mp4 to mp3 (costs one credit)
*/

package main

import (
	"io"
	"math/rand"
	"mime/multipart"
	"os"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserID  int
	EmailID string
	Files   []File
}

type File struct {
	gorm.Model
	UserID    int
	Filename  string
	Extension string
	Data      []byte
	Checksum  string
}

func loadHTML() { // windows support pending
	os := runtime.GOOS
	if os == "linux" {
		cmd := exec.Command("xdg-open", "main.html")
		if err := cmd.Run(); err != nil {
			log.Error(err)
		}
	} else {
		log.Fatal("Unsupported Platform, aborting...")
	}
}

func signUp(emailID string, db *gorm.DB) int {
	min := 1000
	max := 9999
	var randomNumber int

	for {
		randomNumber = rand.Intn(max-min+1) + min
		var user User
		if result := db.First(&user, "ID = ?", randomNumber); result.RowsAffected == 0 {
			break
		}
	}

	newUser := User{
		UserID:  randomNumber,
		EmailID: emailID,
	}

	if err := db.Create(&newUser).Error; err != nil {
		log.Fatal("failed to create new user")
	}
	return randomNumber
}

func constructFileLocally(file *multipart.FileHeader) {
	// Open the uploaded file
	uploadedFile, err := file.Open()
	if err != nil {
		log.Error(err)
	}
	defer uploadedFile.Close()

	// Create the destination file
	destinationPath := file.Filename
	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		log.Error(err)
	}
	defer destinationFile.Close()

	// Copy the contents from the uploaded file to the destination file
	_, err = io.Copy(destinationFile, uploadedFile)
	if err != nil {
		log.Error(err)
	}

}
func pipeline(file *multipart.FileHeader, targetFormat string, userID int) {
	/*
		this function takes a file, a target format and the userID
		first it checks in the db at the Userid if the converted file already exits in it, verifies integrity
		else it uses cloud convert's api to convert it and store it but before converting it scans it using virustotal api
		after the conversion it is stored in the db, and mailed to the user as an attachment
		APIs to use
		1. cloud convert API https://cloudconvert.com/api/v2#overview or https://www.convertapi.com/doc/go-library
		2. Email converted file using https://www.mailjet.com/products/email-api/ or
									  https://sendgrid.com/solutions/email-api/   or
									  https://www.mailersend.com/features/email-api
		file handling https://github.com/spf13/afero
	*/
	/*
		check if targetFormat exists in db for the filename

	*/
	constructFileLocally(file)
}

func buildRouters(router *gin.Engine, db *gorm.DB) {
	router.POST("/upload-by-mail-id", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{
				"error": "File not received",
			})
			return
		}
		toFormat := c.PostForm("to_format")
		emailID := c.PostForm("email_id")
		pipeline(file, toFormat, signUp(emailID, db))
		c.JSON(200, gin.H{
			"message": "File received and processed",
		})
	})

	router.POST("/upload-by-user-id", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{
				"error": "File not received",
			})
			return
		}
		toFormat := c.PostForm("to_format")
		userID_str := c.PostForm("user_id")
		userID_int, err := strconv.Atoi(userID_str)
		if err != nil {
			log.Error(err)
		}
		pipeline(file, toFormat, userID_int)
		c.JSON(200, gin.H{
			"message": "File received and processed",
		})
	})
	if err := router.Run(); err != nil {
		log.Error(err)
	}
}

func main() {
	log.Info("connecting to database...")
	db, err := gorm.Open(sqlite.Open("main.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database")
	}
	err = db.AutoMigrate(&User{})
	if err != nil {
		log.Fatal("database migration failed")
	}

	log.Info("database connected, preparing server...")
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20
	loadHTML()
	buildRouters(router, db)
}
