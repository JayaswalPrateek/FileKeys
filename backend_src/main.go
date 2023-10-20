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

	err = db.AutoMigrate(&Cache{})
	if err != nil {
		log.Fatal("db schema migration failed")
	} else {
		log.Info("db schema migration successful")
	}
	loadHTML()
	return db
}

func loadHTML() {
	if os := runtime.GOOS; os == "linux" {
		cmd := exec.Command("xdg-open", "./../frontend_src/main.html")
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		} else {
			log.Info("Opening main.html")
		}
	} else if os == "windows" {
		cmd := exec.Command("cmd", "/C", "start", "chrome", "main.html")
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		} else {
			log.Info("Opening main.html")
		}
	} else {
		log.Fatal("Unsupported Platform, aborting...")
	}

}

func loadRouter(db *gorm.DB) {
	router := gin.Default()

	router.POST("/upload", func(c *gin.Context) {
		emailID := c.PostForm("mailID")
		log.Info("Received Form Value for email: " + emailID)
		file, err := c.FormFile("uploadedFile")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File upload failed"})
			log.Fatal("File upload failed")
			log.Fatal(err)
		} else {
			log.Info("Uploaded File received")
		}
		fileExtension := filepath.Ext(file.Filename)
		c.JSON(http.StatusOK, gin.H{"message": "File uploaded and processing started"})
		pipeline(file, emailID, db, fileExtension)
	})

	if err := router.Run(":8080"); err != nil {
		log.Fatal("Couldn't spin router on port 8080")
	} else {
		log.Info("Router listening on port 8080")
	}
}

func pipeline(file *multipart.FileHeader, emailID string, db *gorm.DB, fileExtension string) {
	uploadedFile, _ := file.Open()
	defer uploadedFile.Close()

	localFile, _ := os.Create("./tmp" + fileExtension)
	defer localFile.Close()

	_, err := io.Copy(localFile, uploadedFile)
	if err != nil {
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

	// Query the database to check if the hash exists
	hashOfUnconvertedFile := computeSHA256Hash("./tmp" + fileExtension)
	var trxn Cache
	result := db.Where(uploadedFileTypeHash+" = ?", hashOfUnconvertedFile).First(&trxn)

	if result.Error == nil { // Hash exists in the database
		convertedFileblob := trxn.Pblob
		if fileExtension == ".pdf" {
			convertedFileblob = trxn.Oblob
		}

		convertedLocalFile, err := os.Create("./tmp" + targetExtension)
		if err != nil {
			log.Fatal("Couldn't create an empty local file")
		}
		defer convertedLocalFile.Close()

		_, err = convertedLocalFile.Write(convertedFileblob)
		if err != nil {
			log.Fatal("Couldn't build local file from cached blob in db")
		}

		mailToUser(emailID, "./tmp"+targetExtension, targetExtension)
	} else if result.Error == gorm.ErrRecordNotFound {
		cmd := exec.Command("cloudconvert", "convert", "-f", targetExtension[1:], "tmp"+fileExtension)
		cmd.Env = append(os.Environ(), "CLOUDCONVERT_API_KEY=eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJhdWQiOiIxIiwianRpIjoiZWMxYjQ4Mjg4OWY5YWFmZGNkNjYyMTkzN2RlNzg0MzhmNzRhYTUwYzVkMDcxODZkYjgzNDU5OTc4MmE2ZTA3NDQxMWQ5OWE1MTkwODA5NWEiLCJpYXQiOjE2OTc3NzAzNDYuNTEwOTY5LCJuYmYiOjE2OTc3NzAzNDYuNTEwOTcsImV4cCI6NDg1MzQ0Mzk0Ni41MDQ2NDgsInN1YiI6IjY1NzMwNzE3Iiwic2NvcGVzIjpbInByZXNldC53cml0ZSIsInByZXNldC5yZWFkIiwid2ViaG9vay53cml0ZSIsIndlYmhvb2sucmVhZCIsInRhc2sud3JpdGUiLCJ0YXNrLnJlYWQiLCJ1c2VyLndyaXRlIiwidXNlci5yZWFkIl19.Sv_bG0P8H3KX5zvxGaFXfUvHUJQwQSZtSk2INM2omZzfZN_AK-pQ0_ThooN6GkWhb2LZHtXTcj8rKGWt7pb2uQf2uOFYkd3H2k89eQ-70RkIVL2brXtrmd_VAniQ-TE65UNe4xj59CMB1OUaVLMPgVbJQBA7Mb26jQPrEJKmsOHbtfd6avlU4vg5DNwlbbOHQFOhoQ9ke3jWJwn-OjbrfpjskyCR3lR0PZKstPAuEy9JnM0rkTSWZ8dxmW4r1_5Qf1tMnd-6VgH3z7dyT3iAtC3D88IrrpP_Mdo_mR0UYtdUsWS6EFjiqO58-uTI90Lojn9Q-ke7enx5zXSm1DOShk5r8A1kBu9cSulnaGyXiwVWVYRRwOdy4leQHY9735XFzGuqi02DxUvP-dglWwdlFbj3qQm8WgCOSDHgy5TwYfEIjamNqO-Yt5jgQzQcD-WA2NLrk6t7z_9VeaI5y7KTgwAzFGQXzcEdX7Dc0q_Z3sye7HBy1Ppbei0xywg2gUp4aaYhUie1Oa5wkLaLSIdLWoyZjKyum-pbjCdqJyfCbi3dcmfy571b2Pqxy3209-LHeb-bAaB3zQCNkja10eTi9wKnXbfAxfvGaeVluqkJo5J7g0YnaCMdWgqLRanPPiD5WSODhU8fLHJWNnctIkW46sGJ2JBTUlyA16ZLcrVfGHw")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			log.Fatal("Couldn't run cloudconvert's npm tool")
		}

		// file has been converted locally and needs to be cached before it can be mailed
		hashOfConvertedFile := computeSHA256Hash("./tmp" + targetExtension)
		convertedFileBlob, err := os.ReadFile("./tmp" + targetExtension)
		if err != nil {
			log.Fatal("Error reading file while finding hash")
		}

		var newRecord Cache
		uncovertedFileBlob, _ := os.ReadFile("./tmp" + fileExtension)
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

		mailToUser(emailID, "./tmp"+targetExtension, targetExtension)
		os.Remove("./tmp" + targetExtension)
	} else {
		log.Fatal("Error Reading cached entries from db")
	}

	os.Remove("./tmp" + fileExtension)
}
func computeSHA256Hash(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Couldn't open " + filePath)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal("Opened but couldn't hash")
	}

	hashSum := hash.Sum(nil)
	hashString := hex.EncodeToString(hashSum)
	return hashString
}
func mailToUser(emailID string, convertedFileName string, fileExtension string) {
	mailjetClient := mailjet.NewMailjetClient(os.Getenv("MJ_APIKEY_PUBLIC"), os.Getenv("MJ_APIKEY_PRIVATE"))
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
			Subject:  "Your converted file is here!",
			TextPart: "Thank You for using File Keys - A Decentralised File Converter",
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
