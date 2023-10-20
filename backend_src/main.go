package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
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

type trxnHistory struct {
	gorm.Model
	pdfBlob    []byte
	pdfHash    string
	officeBlob []byte
	officeHash string
}

func loadDB() *gorm.DB {
	log.Info("connecting to database...")
	db, err := gorm.Open(sqlite.Open("main.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("database connection failed")
	}
	err = db.AutoMigrate(&trxnHistory{})
	if err != nil {
		log.Fatal("db schema migration failed")
	}
	log.Info("database connected, preparing server...")
	return db
}

func loadRouter(db *gorm.DB) (*multipart.FileHeader, string, *gorm.DB, string) {
	router := gin.Default()
	var emailID, fileExtension string
	var file *multipart.FileHeader
	router.POST("/upload", func(c *gin.Context) {
		emailID = c.PostForm("mailID")
		file, _ = c.FormFile("uploadedFile")
		fileExtension = filepath.Ext(file.Filename)
	})

	return file, emailID, db, fileExtension
}

func pipeline(file *multipart.FileHeader, emailID string, db *gorm.DB, fileExtension string) {
	uploadedFile, _ := file.Open()
	defer uploadedFile.Close()

	localFile, _ := os.Create("./tmp." + fileExtension)
	defer localFile.Close()

	io.Copy(localFile, uploadedFile)
	hashOfLocalFile, _ := computeSHA256Hash("./tmp" + fileExtension)

	var trxn trxnHistory

	// Determine the field to check based on fileExtension
	fieldToCheck := "pdfHash"
	if fileExtension != ".pdf" {
		fieldToCheck = "officeHash"
	}
	var targetExtension string
	switch fileExtension {
	case ".pdf":
		targetExtension = "docx"
	default:
		targetExtension = "pdf"
	}
	// Query the database to check if the hash exists
	result := db.Where(fieldToCheck+" = ?", hashOfLocalFile).First(&trxn)
	if result.Error == nil {
		// Hash exists in the database

		blob := trxn.pdfBlob // Assuming pdfBlob is the default blob to return
		if fileExtension != ".pdf" {
			blob = trxn.officeBlob
		}
		convertedLocalFile, _ := os.Create("./tmp" + targetExtension)
		defer convertedLocalFile.Close()
		_, err := convertedLocalFile.Write(blob)
		if err != nil {
			log.Error(err)
			return
		}
		mailToUser(emailID, "./tmp"+targetExtension, targetExtension)
	} else if result.Error == gorm.ErrRecordNotFound {
		// Define the command and its arguments
		cmd := exec.Command("cloudconvert", "convert", "-f", targetExtension, "tmp"+fileExtension)

		// Set up environment variables if necessary
		cmd.Env = append(os.Environ(), "CLOUDCONVERT_API_KEY=eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJhdWQiOiIxIiwianRpIjoiZWMxYjQ4Mjg4OWY5YWFmZGNkNjYyMTkzN2RlNzg0MzhmNzRhYTUwYzVkMDcxODZkYjgzNDU5OTc4MmE2ZTA3NDQxMWQ5OWE1MTkwODA5NWEiLCJpYXQiOjE2OTc3NzAzNDYuNTEwOTY5LCJuYmYiOjE2OTc3NzAzNDYuNTEwOTcsImV4cCI6NDg1MzQ0Mzk0Ni41MDQ2NDgsInN1YiI6IjY1NzMwNzE3Iiwic2NvcGVzIjpbInByZXNldC53cml0ZSIsInByZXNldC5yZWFkIiwid2ViaG9vay53cml0ZSIsIndlYmhvb2sucmVhZCIsInRhc2sud3JpdGUiLCJ0YXNrLnJlYWQiLCJ1c2VyLndyaXRlIiwidXNlci5yZWFkIl19.Sv_bG0P8H3KX5zvxGaFXfUvHUJQwQSZtSk2INM2omZzfZN_AK-pQ0_ThooN6GkWhb2LZHtXTcj8rKGWt7pb2uQf2uOFYkd3H2k89eQ-70RkIVL2brXtrmd_VAniQ-TE65UNe4xj59CMB1OUaVLMPgVbJQBA7Mb26jQPrEJKmsOHbtfd6avlU4vg5DNwlbbOHQFOhoQ9ke3jWJwn-OjbrfpjskyCR3lR0PZKstPAuEy9JnM0rkTSWZ8dxmW4r1_5Qf1tMnd-6VgH3z7dyT3iAtC3D88IrrpP_Mdo_mR0UYtdUsWS6EFjiqO58-uTI90Lojn9Q-ke7enx5zXSm1DOShk5r8A1kBu9cSulnaGyXiwVWVYRRwOdy4leQHY9735XFzGuqi02DxUvP-dglWwdlFbj3qQm8WgCOSDHgy5TwYfEIjamNqO-Yt5jgQzQcD-WA2NLrk6t7z_9VeaI5y7KTgwAzFGQXzcEdX7Dc0q_Z3sye7HBy1Ppbei0xywg2gUp4aaYhUie1Oa5wkLaLSIdLWoyZjKyum-pbjCdqJyfCbi3dcmfy571b2Pqxy3209-LHeb-bAaB3zQCNkja10eTi9wKnXbfAxfvGaeVluqkJo5J7g0YnaCMdWgqLRanPPiD5WSODhU8fLHJWNnctIkW46sGJ2JBTUlyA16ZLcrVfGHw")

		// Set the standard output and error to os.Stdout and os.Stderr to see the command output
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Execute the command
		err := cmd.Run()
		if err != nil {
			fmt.Println("Error:", err)
		}

		// file has been converted locally and needs to be cached before it can be mailed
		hashOfConvertedFile, _ := computeSHA256Hash("./tmp" + targetExtension)
		targetBlob, err := os.ReadFile("./tmp" + targetExtension)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			return
		}

		var newRecord trxnHistory
		uncovertedFileBlob, _ := os.ReadFile("./tmp" + fileExtension)
		if targetExtension == ".pdf" {
			newRecord.pdfBlob = targetBlob
			newRecord.pdfHash = hashOfConvertedFile
			newRecord.officeBlob = uncovertedFileBlob
			newRecord.officeHash = hashOfLocalFile
		} else {
			newRecord.officeBlob = targetBlob
			newRecord.officeHash = hashOfConvertedFile
			newRecord.pdfBlob = uncovertedFileBlob
			newRecord.pdfHash = hashOfLocalFile
		}
		result := db.Create(&newRecord)
		if result.Error != nil {
			panic("Failed to insert the record")
		}
		convertedLocalFile, _ := os.Create("./tmp" + targetExtension)
		defer convertedLocalFile.Close()
		mailToUser(emailID, "./tmp"+targetExtension, targetExtension)
	} else {
		log.Error(result.Error)
	}

	os.Remove("./tmp." + fileExtension)
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
func mailToUser(emailID string, convertedFileName string, fileExtension string) {
	mailjetClient := mailjet.NewMailjetClient(os.Getenv("MJ_APIKEY_PUBLIC"), os.Getenv("MJ_APIKEY_PRIVATE"))
	content, err := os.ReadFile(convertedFileName)
	if err != nil {
		log.Error(err)
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
		log.Fatal(err)
	}
	fmt.Printf("Data: %+v\n", res)
}

func main() {
	db := loadDB()
	loadHTML()
	pipeline(loadRouter(db))
}
