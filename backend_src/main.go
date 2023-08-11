package main

import (
	"math/rand"
	"mime/multipart"
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
	userID_emailID_mapping        map[int]string
	filename_ext_checksum_mapping map[string]map[string]string
	cheksum_file_mapping          map[string]*multipart.FileHeader
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
		userID_emailID_mapping:        map[int]string{randomNumber: emailID},
		filename_ext_checksum_mapping: make(map[string]map[string]string),
	}

	if err := db.Create(&newUser).Error; err != nil {
		log.Fatal("failed to create new user")
	}
	return randomNumber
}

func pipeline(file *multipart.FileHeader, targetFormat string, userID int) {
	/*
		this function takes a file, a target format and the userID
		first it checks in the db at the Userid if the converted file already exits in it
		else it uses cloud convert's api to convert it and store it but before converting it scans it using virustotal api
		after the conversion it is stored in the db, and mailed to the user as an attachment
		APIs to use
		1. cloud convert API https://cloudconvert.com/api/v2#overview
		2. VirusTotal file scanning API https://developers.virustotal.com/v2.0/reference/getting-started
		3. Email converted file using API(if 1,2,3 goes well) https://github.com/public-apis/public-apis#email
		file handling https://github.com/spf13/afero
	*/
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
	loadHTML()
	buildRouters(router, db)
}
