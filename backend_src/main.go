package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	mailjet "github.com/mailjet/mailjet-apiv3-go"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Cache struct {
	gorm.Model
	Pblob []byte
	Phash string
	Oblob []byte
	Ohash string
}

func connectDB() *gorm.DB {
	log.Info("connecting to database...")

	db, err := gorm.Open(sqlite.Open("main.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("database connection failed")
	} else {
		log.Info("database connection successful")
	}

	if err = db.AutoMigrate(&Cache{}); err != nil {
		log.Fatal("db schema migration failed")
	} else {
		log.Info("db schema migration successful")
	}
	openBrowser()
	return db
}

func openBrowser() {
	var cmd *exec.Cmd

	if runtime.GOOS == "linux" {
		cmd = exec.Command("xdg-open", "http://localhost:8080")
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", "start", "http://localhost:8080")
	} else {
		log.Error("Unsupported Platform, cannot open the browser")
	}

	if err := cmd.Start(); err != nil {
		log.Fatal("Couldn't open localhost url on port 8080")
	}
}

func loadRouter(db *gorm.DB) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("/", func(c *gin.Context) {
		htmlContent, err := byteify("./../frontend_src/main.html")
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "HTML file not found"})
			log.Fatal("Couldn't serve html page on /")
		}
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, string(htmlContent))
	})
	router.POST("/", func(c *gin.Context) {
		emailID := c.PostForm("mailID")
		// log.Info("Received Form Value for email: " + emailID)
		file, err := c.FormFile("uploadedFile")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File upload failed"})
			log.Fatal("File upload failed")
		} else {
			log.Info("Uploaded File received")
		}
		fileExtension := filepath.Ext(file.Filename)
		log.Info("File Extension: " + fileExtension)
		pipeline(file, emailID, db, fileExtension)
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Couldn't spin router on port 8080")
	} else {
		log.Info("Router listening on port 8080")
	}
}
func byteify(filename string) ([]byte, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func pipeline(file *multipart.FileHeader, emailID string, db *gorm.DB, fileExtension string) {
	uploadedFile, _ := file.Open()
	defer uploadedFile.Close()
	unconvertedFile, _ := os.Create("FileKeys" + fileExtension)
	defer unconvertedFile.Close()
	if _, err := io.Copy(unconvertedFile, uploadedFile); err != nil {
		log.Fatal("Couldn't reconstruct uploaded file locally")
	}

	uploadedFileTypeHash := "Ohash"
	if fileExtension == ".pdf" {
		uploadedFileTypeHash = "Phash"
	}
	targetExtension := ".pdf"
	if fileExtension == ".pdf" {
		targetExtension = ".docx"
	}

	unconvertedFileName := "FileKeys" + fileExtension
	convertedFileName := "FileKeys" + targetExtension

	// Query the database to check if the hash exists
	hashOfUnconvertedFile := computeSHA256Hash(unconvertedFileName)
	var trxn Cache
	result := db.Where(uploadedFileTypeHash+" = ?", hashOfUnconvertedFile).First(&trxn)

	if result.Error == nil { // Hash exists in the database
		convertedFileblob := trxn.Pblob
		if fileExtension == ".pdf" {
			convertedFileblob = trxn.Oblob
		}

		convertedLocalFile, err := os.Create(convertedFileName)
		if err != nil {
			log.Fatal("Couldn't create an empty local file")
		}
		defer convertedLocalFile.Close()

		if _, err = convertedLocalFile.Write(convertedFileblob); err != nil {
			log.Fatal("Couldn't build local file from cached blob in db")
		}
		mailToUser(emailID, convertedFileName, targetExtension)
		os.Remove(convertedFileName)
	} else if result.Error == gorm.ErrRecordNotFound {
		cmd := exec.Command("cloudconvert", "convert", "-f", targetExtension[1:], unconvertedFileName)
		cmd.Env = append(os.Environ(), "CLOUDCONVERT_API_KEY=eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJhdWQiOiIxIiwianRpIjoiZWMxYjQ4Mjg4OWY5YWFmZGNkNjYyMTkzN2RlNzg0MzhmNzRhYTUwYzVkMDcxODZkYjgzNDU5OTc4MmE2ZTA3NDQxMWQ5OWE1MTkwODA5NWEiLCJpYXQiOjE2OTc3NzAzNDYuNTEwOTY5LCJuYmYiOjE2OTc3NzAzNDYuNTEwOTcsImV4cCI6NDg1MzQ0Mzk0Ni41MDQ2NDgsInN1YiI6IjY1NzMwNzE3Iiwic2NvcGVzIjpbInByZXNldC53cml0ZSIsInByZXNldC5yZWFkIiwid2ViaG9vay53cml0ZSIsIndlYmhvb2sucmVhZCIsInRhc2sud3JpdGUiLCJ0YXNrLnJlYWQiLCJ1c2VyLndyaXRlIiwidXNlci5yZWFkIl19.Sv_bG0P8H3KX5zvxGaFXfUvHUJQwQSZtSk2INM2omZzfZN_AK-pQ0_ThooN6GkWhb2LZHtXTcj8rKGWt7pb2uQf2uOFYkd3H2k89eQ-70RkIVL2brXtrmd_VAniQ-TE65UNe4xj59CMB1OUaVLMPgVbJQBA7Mb26jQPrEJKmsOHbtfd6avlU4vg5DNwlbbOHQFOhoQ9ke3jWJwn-OjbrfpjskyCR3lR0PZKstPAuEy9JnM0rkTSWZ8dxmW4r1_5Qf1tMnd-6VgH3z7dyT3iAtC3D88IrrpP_Mdo_mR0UYtdUsWS6EFjiqO58-uTI90Lojn9Q-ke7enx5zXSm1DOShk5r8A1kBu9cSulnaGyXiwVWVYRRwOdy4leQHY9735XFzGuqi02DxUvP-dglWwdlFbj3qQm8WgCOSDHgy5TwYfEIjamNqO-Yt5jgQzQcD-WA2NLrk6t7z_9VeaI5y7KTgwAzFGQXzcEdX7Dc0q_Z3sye7HBy1Ppbei0xywg2gUp4aaYhUie1Oa5wkLaLSIdLWoyZjKyum-pbjCdqJyfCbi3dcmfy571b2Pqxy3209-LHeb-bAaB3zQCNkja10eTi9wKnXbfAxfvGaeVluqkJo5J7g0YnaCMdWgqLRanPPiD5WSODhU8fLHJWNnctIkW46sGJ2JBTUlyA16ZLcrVfGHw")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Fatal("Couldn't run cloudconvert's npm tool")
		}

		// file has been converted locally and needs to be cached before it can be mailed
		hashOfConvertedFile := computeSHA256Hash(convertedFileName)
		convertedFileBlob, err := os.ReadFile(convertedFileName)
		if err != nil {
			log.Fatal("Error reading file while finding hash")
		}

		var newRecord Cache
		uncovertedFileBlob, _ := os.ReadFile(unconvertedFileName)
		if targetExtension == ".pdf" {
			newRecord.Pblob = convertedFileBlob
			newRecord.Phash = hashOfConvertedFile
			newRecord.Oblob = uncovertedFileBlob
			newRecord.Ohash = hashOfUnconvertedFile
		} else {
			newRecord.Pblob = uncovertedFileBlob
			newRecord.Phash = hashOfUnconvertedFile
			newRecord.Oblob = convertedFileBlob
			newRecord.Ohash = hashOfConvertedFile
		}
		result = db.Create(&newRecord)
		if result.Error != nil {
			log.Fatal("Failed to insert the record")
		}

		mailToUser(emailID, convertedFileName, targetExtension)
		os.Remove(convertedFileName)
	} else {
		log.Fatal("Error Reading cached entries from db")
	}

	os.Remove(unconvertedFileName)
}

func computeSHA256Hash(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Couldn't open " + filePath)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal("Opened but couldn't hash the file")
	}

	hashSum := hash.Sum(nil)
	hashString := hex.EncodeToString(hashSum)
	return hashString
}

func mailToUser(emailID string, convertedFileName string, fileExtension string) {
	mailjetClient := mailjet.NewMailjetClient("284165cb51dbff7d2706b0eb21167f22", "01abfa596a09fcdef91436c8faa9f8d6")
	content, err := os.ReadFile(convertedFileName)
	if err != nil {
		log.Fatal("Couldn't attach " + convertedFileName + " to mail, error in reading file")
	}

	contentTypeOfFile := "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	if fileExtension == ".pdf" {
		contentTypeOfFile = "application/pdf"
	}

	base64ContentOfFile := base64.StdEncoding.EncodeToString(content)
	messagesInfo := []mailjet.InfoMessagesV31{
		mailjet.InfoMessagesV31{
			From: &mailjet.RecipientV31{
				Email: "filekeysteam@gmail.com",
				Name:  "Prateek",
			},
			To: &mailjet.RecipientsV31{
				mailjet.RecipientV31{
					Email: emailID,
					Name:  "User",
				},
			},
			Subject:  "Your Converted File Is Here!",
			TextPart: "Thank You For Using FileKeys - A Decentralised File Converter",
			HTMLPart: "",
			Attachments: &mailjet.AttachmentsV31{
				mailjet.AttachmentV31{
					ContentType:   contentTypeOfFile,
					Filename:      convertedFileName,
					Base64Content: base64ContentOfFile,
				},
			},
		},
	}

	messages := mailjet.MessagesV31{Info: messagesInfo}
	res, err := mailjetClient.SendMailV31(&messages)
	if err != nil {
		log.Fatal("Couldn't send mail")
	}
	fmt.Printf("Data: %+v\n", res)
}

func main() {
	loadRouter(connectDB())
}
