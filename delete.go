package walg

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"log"
	"strconv"
	"strings"
	"time"
)

const DeleteUsageText = "delete requires at least 2 parameters" + `
		retain 5                      keep 5 backups
		retain FULL 5                 keep 5 full backups and all deltas of them
		retail FIND_FULL 5            find necessary full for 5th and keep everything after it
		before base_0123              keep everything after base_0123 including itself
		before FIND_FULL base_0123    keep everything after the base of base_0123`

// DeleteCommandArguments incapsulates arguments for delete command
type DeleteCommandArguments struct {
	full       bool
	findFull   bool
	retain     bool
	before     bool
	target     string
	beforeTime *time.Time
	dryrun     bool
}

// ParseDeleteArguments interprets arguments for delete command. TODO: use flags or cobra
func ParseDeleteArguments(args []string, fallBackFunc func()) (result DeleteCommandArguments) {
	if len(args) < 3 {
		fallBackFunc()
		return
	}

	params := args[1:]
	if params[0] == "retain" {
		result.retain = true
		params = params[1:]
	} else if params[0] == "before" {
		result.before = true
		params = params[1:]
	} else {
		fallBackFunc()
		return
	}
	if params[0] == "FULL" {
		result.full = true
		params = params[1:]
	} else if params[0] == "FIND_FULL" {
		result.findFull = true
		params = params[1:]
	}
	if len(params) < 1 {
		log.Print("Backup name not specified")
		fallBackFunc()
		return
	}

	result.target = params[0]
	if t, err := time.Parse(time.RFC3339, result.target); err == nil {
		if t.After(time.Now()) {
			log.Println("Cannot delete before future date")
			fallBackFunc()
		}
		result.beforeTime = &t
	}
	//if DeleteConfirmed && !DeleteDryrun  // TODO: use flag
	result.dryrun = true
	if len(params) > 1 && (params[1] == "--confirm" || params[1] == "-confirm") {
		result.dryrun = false
	}

	if result.retain {
		number, err := strconv.Atoi(result.target)
		if err != nil {
			log.Println("Cannot parse target number ", number)
			fallBackFunc()
			return
		}
		if number <= 0 {
			log.Println("Cannot retain 0") // Consider allowing to delete everything
			fallBackFunc()
			return
		}
	}
	return
}

func deleteBeforeTarget(target string, bk *Backup, pre *S3Prefix, findFull bool, backups []BackupTime, dryRun bool) {
	dto := fetchSentinel(target, bk, pre)
	if dto.IsIncremental() {
		if findFull {
			target = *dto.IncrementFullName
		} else {
			log.Fatalf("%v is incemental and it's predecessors cannot be deleted. Consider FIND_FULL option.", target)
		}
	}
	var err error
	if backups == nil {
		backups, err = bk.GetBackups()
		if err != nil {
			log.Fatal(err)
		}
	}


	skipLine, walSkipFileName := ComputeDeletionSkipline(backups, target)

	if !dryRun {
		if skipLine < len(backups)-1 {
			deleteWALBefore(walSkipFileName, pre)
			deleteBackupsBefore(backups, skipLine, pre)
		}
	} else {
		log.Printf("Dry run finished.\n")
	}
}

// ComputeDeletionSkipline selects last backup and name of last necessary WAL
func ComputeDeletionSkipline(backups []BackupTime, target string) (skipLine int, walSkipFileName string) {
	skip := true
	skipLine = len(backups)
	walSkipFileName = ""
	for i, backupTime := range backups {
		if skip {
			log.Printf("%v skipped\n", backupTime.Name)
			if walSkipFileName == "" || walSkipFileName > backupTime.WalFileName {
				walSkipFileName = backupTime.WalFileName
			}
		} else {
			log.Printf("%v will be deleted\n", backupTime.Name)
		}
		if backupTime.Name == target {
			skip = false
			skipLine = i
		}
	}
	return skipLine, walSkipFileName
}

// TODO : unit tests
func deleteBackupsBefore(backups []BackupTime, skipline int, pre *S3Prefix) {
	for i, b := range backups {
		if i > skipline {
			dropBackup(pre, b)
		}
	}
}

func dropBackup(pre *S3Prefix, b BackupTime) {
	var bk = &Backup{
		Prefix: pre,
		Path:   GetBackupPath(pre),
		Name:   aws.String(b.Name),
	}
	tarFiles, err := bk.GetKeys()
	if err != nil {
		log.Fatal("Unable to list backup for deletion ", b.Name, err)
	}

	folderKey := strings.TrimPrefix(*pre.Server+BaseBackupsPath+b.Name, "/")
	suffixKey := folderKey + SentinelSuffix

	keys := append(tarFiles, suffixKey, folderKey)
	parts := partition(keys, 1000)
	for _, part := range parts {

        for _, key := range part {
            input := &s3.DeleteObjectInput{Bucket: pre.Bucket, Key: &key}
            _, err = pre.Svc.DeleteObject(input)
            if err != nil {
                log.Fatal("Unable to delete backup ", b.Name, err)

		    }
		}

	}
}

func partitionToObjects(keys []string) []*s3.ObjectIdentifier {
	objs := make([]*s3.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objs[i] = &s3.ObjectIdentifier{Key: aws.String(k)}
	}
	return objs
}

func deleteWALBefore(walSkipFileName string, pre *S3Prefix) {
	var bk = &Backup{
		Prefix: pre,
		Path:   aws.String(sanitizePath(*pre.Server + WalPath)),
	}

	objects, err := bk.GetWals(walSkipFileName)
	if err != nil {
		log.Fatal("Unable to obtaind WALS for border ", walSkipFileName, err)
	}
	parts := partitionObjects(objects, 1000)
	for _, part := range parts {
        for _, key := range part {
            input := &s3.DeleteObjectInput{Bucket: pre.Bucket, Key: key.Key}
            _, err = pre.Svc.DeleteObject(input)
            if err != nil {
                log.Fatal("Unable to delete WALS before ", walSkipFileName, err)
            }
		}
	}
}

func printDeleteUsageAndFail() {
	log.Fatal(DeleteUsageText)
}
