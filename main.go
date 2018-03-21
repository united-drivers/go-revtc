package main

import (
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseUrl = "https://registre-vtc.developpement-durable.gouv.fr/public"

type APIPersonTitle int
type APILegalEntityType int
type APIBusinessEntityType int
type APISearchParams int

const (
	personTitleOther APIPersonTitle = iota
	personTitleMr
	personTitleMrs
)

const (
	legalEntityTypeOther APILegalEntityType = iota
	legalEntityTypeCompany
	legalEntityTypeIndividual
)

const (
	businessEntityTypeOther APIBusinessEntityType = iota
	businessEntityTypeSA
	businessEntityTypeSARL
	businessEntityTypeSAS
	businessEntityTypeSASU
	businessEntityTypeEURL
)

const (
	lCompanyName         = "Dénomination"
	lCompanyNumber       = "Numéro SIREN"
	lRegistrationNumber  = "Numéro d'inscription"
	lContactFirstName    = "Prénom"
	lContactLastName     = "Nom"
	lCity                = "Ville"
	lAcronym             = "Sigle"
	lExpirationDate      = "Valide jusqu'au"
	lLegalEntityType     = "Statut"
	lCompanyType         = "Forme juridique"
	lBrand               = "Marque/Nom commercial"
	lPostalCode          = "Code Postal"
	lDepartment          = "Département"
	lCountry             = "Pays"
	lIndividualTitle     = "Civilité"
	lIndividualFirstName = "Prénom principal"
	lIndividualLastName  = "Nom d'usage"
)

const (
	sRegistrationNumber APISearchParams = iota
	sCompanyNumber
	sPersonName
	sCompanyName
	sAcronym
	sBrand
	sCity
	sPostalCode
	sDepartment
)

type APIResultAddress struct {
	PostalCode string `json:"postal_code,omitempty"`
	City       string `json:"city,omitempty"`
	Country    string `json:"country,omitempty"`
	Department string `json:"department,omitempty"`
}

type APIPersonName struct {
	LastName  string `json:"last_name,omitempty"`
	FirstName string `json:"first_name,omitempty"`
}

type APIResultIndividual struct {
	Title APIPersonTitle `json:"title,omitempty"`
	Name  APIPersonName  `json:"name,omitempty"`
}

type APIResultCompany struct {
	Name        string                `json:"name,omitempty"`
	Acronym     string                `json:"acronym,omitempty"`
	Brand       string                `json:"brand,omitempty"`
	Contact     APIPersonName         `json:"contact,omitempty"`
	CompanyType APIBusinessEntityType `json:"company_type,omitempty"`
}

type APIResult struct {
	LegalEntityType    APILegalEntityType  `json:"legal_entity_type"`
	CompanyNumber      string              `json:"company_number"`
	RegistrationNumber string              `json:"registration_number"`
	ExpirationDate     time.Time           `json:"expiration_date"`
	Address            APIResultAddress    `json:"address"`
	Individual         APIResultIndividual `json:"individual,omitempty"`
	Company            APIResultCompany    `json:"company,omitempty"`
}

var personTitleMapping = []string{
	personTitleMr:  "M.",
	personTitleMrs: "Mme",
}

var businessEntityTypeMapping = []string{
	businessEntityTypeSA:   "Société anonyme",
	businessEntityTypeSARL: "Société à responsabilité limitée",
	businessEntityTypeSAS:  "Société par actions simplifiée",
	businessEntityTypeSASU: "Société par actions simplifiée unipersonnelle",
	businessEntityTypeEURL: "Entreprise unipersonnelle à responsabilité limitée",
}

var legalEntityTypeMapping = []string{
	legalEntityTypeCompany:    "Personne morale",
	legalEntityTypeIndividual: "Personne physique",
}

var personTitleJSON = []string{
	personTitleOther: "OTHER",
	personTitleMr:    "MR",
	personTitleMrs:   "MRS",
}

var businessEntityTypeJSON = []string{
	businessEntityTypeOther: "OTHER",
	businessEntityTypeSA:    "SA",
	businessEntityTypeSARL:  "SARL",
	businessEntityTypeSAS:   "SAS",
	businessEntityTypeSASU:  "SASU",
	businessEntityTypeEURL:  "EURL",
}

var legalEntityTypeJSON = []string{
	legalEntityTypeOther:      "OTHER",
	legalEntityTypeCompany:    "COMPANY",
	legalEntityTypeIndividual: "INDIVIDUAL",
}

func (value APIPersonTitle) MarshalText() ([]byte, error) {
	return []byte(personTitleJSON[value]), nil
}

func (value APILegalEntityType) MarshalText() ([]byte, error) {
	return []byte(legalEntityTypeJSON[value]), nil
}

func (value APIBusinessEntityType) MarshalText() ([]byte, error) {
	return []byte(businessEntityTypeJSON[value]), nil
}

func getKeyForMappingValue(mapping []string, inputValue string, defaultValue int) int {
	for key, value := range mapping {
		if value == inputValue {
			return key
		}
	}

	return defaultValue
}

func castAPIPersonTitle(str string) APIPersonTitle {
	return APIPersonTitle(getKeyForMappingValue(personTitleMapping, str, int(personTitleOther)))
}

func castAPIBusinessEntityType(str string) APIBusinessEntityType {
	return APIBusinessEntityType(getKeyForMappingValue(businessEntityTypeMapping, str, int(businessEntityTypeOther)))
}

func castAPILegalEntityType(str string) APILegalEntityType {
	return APILegalEntityType(getKeyForMappingValue(legalEntityTypeMapping, str, int(legalEntityTypeOther)))
}

func mapDictToObject(mapped map[string]string) APIResult {
	var result APIResult

	result.CompanyNumber = mapped[lCompanyNumber]
	result.RegistrationNumber = mapped[lRegistrationNumber]

	result.Address.City = mapped[lCity]
	result.Address.Country = mapped[lCountry]
	result.Address.PostalCode = mapped[lPostalCode]
	result.Address.Department = mapped[lDepartment]

	result.LegalEntityType = castAPILegalEntityType(mapped[lLegalEntityType])

	if result.LegalEntityType == legalEntityTypeCompany {
		result.Company.Name = mapped[lCompanyName]
		result.Company.Acronym = mapped[lAcronym]
		result.Company.Brand = mapped[lBrand]
		result.Company.Contact.FirstName = mapped[lContactFirstName]
		result.Company.Contact.LastName = mapped[lContactLastName]
		result.Company.CompanyType = castAPIBusinessEntityType(mapped[lCompanyType])
		result.Company.Brand = mapped[lBrand]
		result.Company.Acronym = mapped[lAcronym]

	} else if result.LegalEntityType == legalEntityTypeIndividual {
		result.Individual.Title = castAPIPersonTitle(mapped[lIndividualTitle])
		result.Individual.Name.FirstName = mapped[lIndividualFirstName]
		result.Individual.Name.LastName = mapped[lIndividualLastName]
	}

	fmt.Println(mapped[lExpirationDate])

	result.ExpirationDate, _ = time.Parse(
		"02/01/2006", mapped[lExpirationDate])

	return result
}

func handleSingleResultPage(res *http.Response) (APIResult, error) {
	if res.StatusCode != 200 {
		return APIResult{}, errors.New("not found")
	}

	doc, errHtml := html.Parse(res.Body)
	sel, errCss := cascadia.Compile(".cLabel")

	if errHtml != nil {
		return APIResult{}, errHtml
	}

	if errCss != nil {
		return APIResult{}, errCss
	}

	mapped := map[string]string{}

	results := sel.MatchAll(doc)

	for _, node := range results {
		parent := node.Parent

		// edge case, in one of the tables labels are wrapped in a <span> elt
		if parent.Data == "span" {
			parent = parent.Parent
		}

		value := getTextToken(parent)

		mapped[getTextToken(node)] = value
	}

	if mapped[lCompanyNumber] == "" {
		return APIResult{}, errors.New("not found")
	}

	return mapDictToObject(mapped), nil
}

func getTextToken(node *html.Node) string {
	subNode := node.FirstChild

	for subNode != nil {
		if subNode.Type == html.TextNode {
			value := strings.TrimSpace(subNode.Data)

			if value != "" {
				return value
			}
		}

		subNode = subNode.NextSibling
	}

	return ""
}

func GetByRecordId(recordId int) (APIResult, error) {
	var requestUrl = fmt.Sprintf(
		"%s/rechercheExploitant.exploitantDetails.action?dossier.id=%d",
		baseUrl, recordId)

	resp, err := http.Get(requestUrl)

	if err != nil {
		return APIResult{}, err
	}

	return handleSingleResultPage(resp)
}

func GetByAdvancedSearch(params map[APISearchParams]string) (APIResult, error) {
	var requestUrl = fmt.Sprintf(
		"%s/rechercheExploitant.avancee.action", baseUrl)

	resp, err := http.PostForm(requestUrl, url.Values{
		"rechercheCriteres.numeroInscription":              {params[sRegistrationNumber]},
		"rechercheCriteres.nomRepresentantLegal":           {params[sPersonName]},
		"rechercheCriteres.nomDenomination":                {params[sCompanyName]},
		"rechercheCriteres.numeroSiren":                    {params[sCompanyNumber]},
		"rechercheCriteres.sigle":                          {params[sAcronym]},
		"rechercheCriteres.marque":                         {params[sBrand]},
		"rechercheCriteres.autreFormeJuridique":            {""},
		"rechercheCriteres.idFormeJuridique":               {""},
		"rechercheCriteres.ville":                          {params[sCity]},
		"rechercheCriteres.idPays":                         {""},
		"rechercheCriteres.codePostal":                     {params[sPostalCode]},
		"rechercheCriteres.idRegion":                       {""},
		"rechercheCriteres.idDepartement":                  {params[sDepartment]},
		"action:/public/rechercheExploitant.liste.avancee": {"Rechercher"},
	})

	if err != nil {
		return APIResult{}, err
	}

	return handleSingleResultPage(resp)
}

func GetByCompanyNumber(companyNumber string) (APIResult, error) {
	return GetByAdvancedSearch(map[APISearchParams]string{
		sCompanyNumber: companyNumber,
	})
}

func GetByRegistrationNumber(registrationNumber string) (APIResult, error) {
	return GetByAdvancedSearch(map[APISearchParams]string{
		sRegistrationNumber: registrationNumber,
	})
}

func httpSimpleSearch(c *gin.Context, searchType APISearchParams) {
	input := c.Param("input")
	result, err := GetByAdvancedSearch(map[APISearchParams]string{
		searchType: input,
	})

	if err != nil {
		c.JSON(400, gin.H{
			"message": string(err.Error()),
		})

		return
	}

	c.JSON(http.StatusOK, result)
}

func httpSearchByRegNumber(c *gin.Context) {
	httpSimpleSearch(c, sRegistrationNumber)
}

func httpSearchByCompanyNumber(c *gin.Context) {
	httpSimpleSearch(c, sCompanyNumber)
}

func main() {
	r := gin.Default()

	r.GET("/registration_number/:input", httpSearchByRegNumber)
	r.GET("/company_number/:input", httpSearchByCompanyNumber)

	r.Run() // listen and serve on 0.0.0.0:8080
}
