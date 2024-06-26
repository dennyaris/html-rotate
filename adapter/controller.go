package adapter

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"log"
	"math"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	BuilderQuery "github.com/dennyaris/html-rotate/package"
	"github.com/dennyaris/html-rotate/util"
)

type PageData struct {
	PageID  string `json:"pageID"`
	AdsName string `json:"adsName"`
}

type DataSampling struct {
	VariantID  string
	Success    int
	Impression int
}

// AlphaBeta represents alpha and beta values for a variant
type AlphaBeta struct {
	Alpha int
	Beta  int
}

type ReturnGetObjective struct {
	SelectedVariant string
	Objective       string
}

type rotatorData struct {
	SelectedVariant string `json:"selectedVariant"`
	PageID          string `json:"pageID"`
}

var newDat PageData

func RotateHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) error {
	url := strings.TrimSpace(r.URL.Query().Get("url"))
	adsName := strings.TrimSpace(r.URL.Query().Get("ads"))

	if url == "" {
		util.ResponseError(w, "params url is empty!", http.StatusBadRequest)
		return nil
	}
	if adsName == "" {
		util.ResponseError(w, "params ads is empty!", http.StatusBadRequest)
		return nil
	}

	value, err := util.GetMemcachedValue("page_db_stat")
	if err != nil {
		log.Printf("error getting memcached : %v\n", err)
	}

	pageType, pageID, err := getPageFromDB(db, url)
	if err != nil {
		log.Printf("error getting page : %v", err)
		return err
	}

	if value == nil {
		newVal := PageData{
			PageID:  pageID,
			AdsName: adsName,
		}

		jsonData, err := json.Marshal(newVal)
		if err != nil {
			log.Printf("error json : %v", err)
			return err
		}
		if err = util.SetMemcachedValue("page_db_stat", jsonData, 60); err != nil {
			log.Printf("error set memcached : %v", err)
			return err
		}
	} else {
		err := json.Unmarshal(value, &newDat)
		if err != nil {
			log.Printf("error unmarshal : %v", err)
			return err
		}

		log.Printf("value from memcache : %v", string(value))
		pageID = newDat.PageID
		adsName = newDat.AdsName
	}

	if pageType == "rotator" {
		selectedVariant, err := rotatorGetPage(db, pageID, adsName)
		if err != nil {
			log.Printf("Error getting rotator page : %v", err)
			return err
		}

		util.ResponseSuccess(w, rotatorData{
			SelectedVariant: selectedVariant,
			PageID:          pageID,
		}, "")
	}

	return nil
}

func getPageFromDB(db *sql.DB, url string) (string, string, error) {
	var pageType, pageID string
	var isRotator int

	pageQuery := "SELECT page_id, is_rotator FROM page WHERE url_key = UNHEX(?) LIMIT 1"
	rows, err := db.Query(pageQuery, fmt.Sprintf("%x", sha256.Sum256([]byte(url))))
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	if !rows.Next() {
		return "", "", errors.New("no page found for the given url")
	}

	err = rows.Scan(&pageID, &isRotator)
	if err != nil {
		return "", "", err
	}

	if isRotator == 1 {
		pageType = "rotator"
	} else {
		pageType = "page"
	}

	return pageType, pageID, nil
}

func rotatorGetPage(db *sql.DB, rotatorID, adsName string) (string, error) {
	experimentID := strings.ReplaceAll(rotatorID, "r_", "e_") + "_" + adsName

	crc := crc32.ChecksumIEEE([]byte(experimentID))
	tableID := strconv.FormatUint(uint64(crc), 10)[:2]
	println(tableID)

	variantId := ""
	tableName := "z_rotator_variant_history_" + tableID

	hash := sha256.Sum256([]byte(experimentID))
	hashedString := hex.EncodeToString(hash[:])

	vh, err := BuilderQuery.GetVariantHistoryByExperimentKey(db, tableName, hashedString)
	if err != nil {
		log.Fatal(err)
	}

	if vh == nil {
		addExperiment(db, rotatorID, adsName)
		vh, err = BuilderQuery.GetVariantHistoryByExperimentKey(db, tableName, hashedString)
		if err != nil {
			log.Fatal(err)
		}
	}

	objective := getObjective(vh)
	if objective.SelectedVariant != "" {
		variantId = objective.SelectedVariant
	} else {
		data := make(map[string]map[string]int)
		total_reward := make(map[string]int)
		arm_count := make(map[string]int)
		for _, history := range vh {

			v := reflect.ValueOf(history)

			success_value := v.FieldByName(objective.Objective)
			success_value_int := int(success_value.Uint())

			data[history.VariantID] = map[string]int{
				"success":    int(success_value.Uint()), // Assuming Purchase field represents success
				"fail":       int(history.Impression) - success_value_int,
				"impression": int(history.Impression),
			}

			total_reward[history.VariantID] = success_value_int
			arm_count[history.VariantID] = int(history.Impression)
		}

		threshold := calculateThreshold(total_reward, arm_count, 0.95)
		variantId = mabExploit(data)

		if threshold[variantId] <= 0.01 {
			useMAB := randomBoolWithWeight(0.9)
			if !useMAB {
				variantId = bernoulliThompsonSampling(data)
			}
		} else {
			variantId = bernoulliThompsonSampling(data)
		}
	}

	// Get the current date in "Y-m-d" format
	tanggal := time.Now().Format("2006-01-02")

	// Prepare the SQL statement
	query := "INSERT INTO " + tableName + " (tanggal, experiment_id, experiment_key, variant_id, variant_key, impression) VALUES (?, ?, UNHEX(?), ?, UNHEX(?), ?) ON DUPLICATE KEY UPDATE impression = impression + 1"
	stmt, err := db.Prepare(query)
	if err != nil {
		panic(err.Error())
	}
	defer stmt.Close()

	exp_hash := sha256.Sum256([]byte(experimentID))
	exp_hashedString := hex.EncodeToString(exp_hash[:])

	variant_hash := sha256.Sum256([]byte(variantId))
	variant_hashedString := hex.EncodeToString(variant_hash[:])

	// Execute the SQL statement
	_, err = stmt.Exec(tanggal, experimentID, exp_hashedString, variantId, variant_hashedString, 1)
	if err != nil {
		panic(err.Error())
	}

	// for _, result := range results {
	// 	fmt.Printf("Variant ID: %s\n", result.VariantID)
	// 	fmt.Printf("Experiment ID: %s\n", result.ExperimentID)
	// 	fmt.Printf("Tanggal: %s\n", result.Tanggal)
	// 	fmt.Printf("Impression: %d\n", result.Impression)
	// 	fmt.Printf("CTA: %d\n", result.CTA)
	// 	fmt.Println("")
	// 	fmt.Println("")
	// }

	return variantId, nil
}

func randomBoolWithWeight(weightTrue float64) bool {
	r := rand.Float64()
	return r < weightTrue
}

// Function to exploit the Multi-Armed Bandit (MAB) algorithm
func mabExploit(data map[string]map[string]int) string {
	rate := make(map[string]float64)

	for key, value := range data {
		rate[key] = float64(value["success"]) / float64(value["impression"])
	}

	var maxValue float64
	var selectedVariant string

	for key, value := range rate {
		if value > maxValue {
			maxValue = value
			selectedVariant = key
		}
	}

	return selectedVariant
}

func addExperiment(db *sql.DB, rotatorID, adsName string) (string, error) {
	experimentID := strings.ReplaceAll(rotatorID, "r_", "e_") + "_" + adsName

	exp_hash := sha256.Sum256([]byte(experimentID))
	exp_hashedString := hex.EncodeToString(exp_hash[:])
	rotator_hash := sha256.Sum256([]byte(experimentID))
	rotator_hashedString := hex.EncodeToString(rotator_hash[:])

	// Data to be inserted
	dataExperiment := map[string]string{
		"experiment_id":  experimentID,
		"experiment_key": exp_hashedString,
		"rotator_id":     rotatorID,
		"rotator_key":    rotator_hashedString,
		"ads_name":       adsName,
	}

	successful, err := BuilderQuery.InsertIntoTable(db, "z_rotator_experiment", dataExperiment)
	if err != nil {
		log.Fatal(err)
	}

	if !successful {
		///////////////////////// get all pages attach to this rotator add to variant and variant history ///////////
		rotator, err := BuilderQuery.GetPagesByRotatorKey(db, rotator_hashedString)
		if err != nil {
			log.Fatal(err)
		}

		for _, page := range rotator {
			AddVariant(db, experimentID, page.PageID)
			AddVariantHistory(db, experimentID, page.PageID, "")
		}
	} else {
		row, err := BuilderQuery.SelectFromZRotatorExperiment(db, exp_hashedString)
		if err != nil {
			log.Fatal(err)
		}

		AddVariant(db, experimentID, rotatorID)
		AddVariantHistory(db, experimentID, rotatorID, "")
		experimentID = row.ExperimentID
	}

	return experimentID, err
}

func AddVariant(db *sql.DB, experimentID, pageID string) (string, error) {
	pageIDParts := strings.Split(pageID, "_")
	pageID = pageIDParts[len(pageIDParts)-1]

	variantID := strings.ReplaceAll(experimentID, "e_", "v_") + "_" + strings.ReplaceAll(pageID, "p_", "")

	query := "INSERT IGNORE INTO z_rotator_variant (variant_id, variant_key, experiment_id, experiment_key, page_id, page_key) VALUES (?, UNHEX(?), ?, UNHEX(?), ?, UNHEX(?))"
	variantKey := fmt.Sprintf("%x", sha256.Sum256([]byte(variantID)))

	_, err := db.Exec(query, variantID, variantKey, experimentID, fmt.Sprintf("%x", sha256.Sum256([]byte(experimentID))), pageID, fmt.Sprintf("%x", sha256.Sum256([]byte(pageID))))
	if err != nil {
		return "", err
	}

	return variantID, nil
}

func AddVariantHistory(db *sql.DB, experimentID, pageID string, tanggal string) (string, error) {
	var tanggalStr string
	if len(tanggal) == 0 {
		tanggalStr = time.Now().Format("2006-01-02")
	} else {
		tanggalStr = tanggal
	}

	pageIDParts := strings.Split(pageID, "_")
	pageID = pageIDParts[len(pageIDParts)-1]

	variantID := strings.ReplaceAll(experimentID, "e_", "v_") + "_" + strings.ReplaceAll(pageID, "p_", "")

	crc := crc32.ChecksumIEEE([]byte(experimentID))
	tableID := strconv.FormatUint(uint64(crc), 10)[:2]

	query := "INSERT IGNORE INTO z_rotator_variant_history_" + tableID + " (variant_id, variant_key, experiment_id, experiment_key, tanggal) VALUES (?, UNHEX(?), ?, UNHEX(?), ?)"

	variantKey := fmt.Sprintf("%x", sha256.Sum256([]byte(variantID)))

	_, err := db.Exec(query, variantID, variantKey, experimentID, fmt.Sprintf("%x", sha256.Sum256([]byte(experimentID))), tanggalStr)
	if err != nil {
		return "", err
	}

	return variantID, nil
}

func getObjective(data []BuilderQuery.VariantHistory) ReturnGetObjective {
	objective := "CTA"
	isLead := true
	isMql := true
	isProspek := true
	isPurchase := true

	selectedVariant := ""

	for _, value := range data {
		if value.CTA < 1 {
			selectedVariant = value.VariantID
		}
		if value.Lead < 1 {
			isLead = false
		}
		if value.Mql < 1 {
			isMql = false
		}
		if value.Prospek < 1 {
			isProspek = false
		}
		if value.Purchase < 1 {
			isPurchase = false
		}
	}

	if isPurchase {
		objective = "Purchase"
	} else if isProspek {
		objective = "Prospek"
	} else if isMql {
		objective = "Mql"
	} else if isLead {
		objective = "Lead"
	}

	return ReturnGetObjective{SelectedVariant: selectedVariant, Objective: objective}
}

// Function to perform Bernoulli Thompson Sampling
func bernoulliThompsonSampling(data map[string]map[string]int) string {
	alpha := make(map[string]float64)
	beta := make(map[string]float64)

	// Calculate alpha and beta values for each variant
	for variantID, row := range data {
		successSum := float64(row["Success"])                  // Convert to float64
		failSum := float64(row["Impression"] - row["Success"]) // Convert to float64

		// Update alpha and beta maps for the variant
		alpha[variantID] += successSum
		beta[variantID] += failSum
	}

	// Calculate sampled values using beta distribution
	sampledValues := make(map[string]float64)
	for variantID := range alpha {
		// Convert alpha and beta values to int before passing to betaDistribution
		sampledValues[variantID] = betaDistribution(int(alpha[variantID]), int(beta[variantID]))
	}

	// Find the selected variant with the maximum sampled value
	var selectedVariant string
	maxSampledValue := 0.0
	for variantID, value := range sampledValues {
		if value > maxSampledValue {
			maxSampledValue = value
			selectedVariant = variantID
		}
	}

	return selectedVariant
}

// betaDistribution calculates the beta distribution value for given alpha and beta
func betaDistribution(alpha, beta int) float64 {
	return float64(rand.Intn(beta+1)) / float64(rand.Intn(alpha+beta+1))
}

// Function to calculate the threshold for each arm
func calculateThreshold(totalRewards map[string]int, armCounts map[string]int, confidenceLevel float64) map[string]float64 {
	thresholds := make(map[string]float64)

	for arm, reward := range totalRewards {
		meanReward := float64(reward) / float64(armCounts[arm])
		standardError := math.Sqrt(meanReward * (1 - meanReward) / float64(armCounts[arm]))
		zScore := math.Abs(statsInv(1 - (confidenceLevel / 2)))
		marginOfError := zScore * standardError
		thresholds[arm] = 2 * marginOfError
	}

	return thresholds
}

// Function to calculate the inverse of the standard normal cumulative distribution function
func statsInv(p float64) float64 {
	q := p - 0.5
	var r float64

	if math.Abs(q) <= 0.425 {
		r = 0.180625 - q*q
		return q * (((((((2.5090809287301226727e3*r+3.3430575583588128105e4)*r+6.7265770927008700853e4)*r+4.5921953931549871457e4)*r+1.3731693765509461125e4)*r+1.9715909503065514427e3)*r+1.3314166789178437745e2)*r + 3.3871328727963666080e0) / (((((((5.2264952788528545610e3*r+2.8729085735721942674e4)*r+3.9307895800092710610e4)*r+2.1213794301586595867e4)*r+5.3941960214247511077e3)*r+6.8718700749205790830e2)*r+4.2313330701600911252e1)*r + 1.0)
	}

	r = math.Log(-math.Log(p))
	r = 1.570796288 + r*(0.305532033+r*(0.0000000383+r*(-0.000003298)))
	if q < 0 {
		return -r
	}
	return r
}
