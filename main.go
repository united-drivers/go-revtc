package main

import (
	"fmt"
	"time"
	"net/http"
	"golang.org/x/net/html"
	"github.com/andybalholm/cascadia"
	"strings"
	"errors"
	"net/url"
)

const baseUrl = "https://registre-vtc.developpement-durable.gouv.fr/public"

type APIEnum int
type APIPersonTitle APIEnum
type APILegalEntityType APIEnum
type APIBusinessEntityType APIEnum
type APISearchParams APIEnum

const (
	personTitleOther APIPersonTitle = iota
	personTitleMr
	personTitleMrs
)

const (
	legalEntityTypeOther      APILegalEntityType = iota
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
)

type APIResultAddress struct {
	postalCode, city, country, department string
}

type APIPersonName struct {
	lastName, firstName string
}

type APIResultIndividual struct {
	title APIPersonTitle
	name  APIPersonName
}

type APIResultCompany struct {
	name, acronym, brand string
	contact              APIPersonName
	companyType          APIBusinessEntityType
}

type APIResult struct {
	legalEntityType                   APILegalEntityType
	companyNumber, registrationNumber string
	expirationDate                    time.Time
	address                           APIResultAddress
	individual                        APIResultIndividual
	company                           APIResultCompany
}

func mapPersonTitle(inputValue string) APIPersonTitle {
	mapping := map[APIPersonTitle]string{
		personTitleMr:  "M.",
		personTitleMrs: "Mme",
	}

	for key, value := range mapping {
		if value == inputValue {
			return key
		}
	}

	return personTitleOther
}

func mapCompanyType(inputValue string) APIBusinessEntityType {
	mapping := map[APIBusinessEntityType]string{
		businessEntityTypeSAS:  "Société par actions simplifiée",
		businessEntityTypeSA:   "Société anonyme",
		businessEntityTypeSARL: "Société à responsabilité limitée",
		businessEntityTypeSASU:
		"Société par actions simplifiée unipersonnelle",
		businessEntityTypeEURL:
		"Entreprise unipersonnelle à responsabilité limitée",
	}

	for key, value := range mapping {
		if value == inputValue {
			return key
		}
	}

	return businessEntityTypeOther
}

func mapLegalEntityType(inputValue string) APILegalEntityType {
	mapping := map[APILegalEntityType]string{
		legalEntityTypeCompany:    "Personne morale",
		legalEntityTypeIndividual: "Personne physique",
	}

	for key, value := range mapping {
		if value == inputValue {
			return key
		}
	}

	return legalEntityTypeOther
}

func mapDictToObject(mapped map[string]string) APIResult {
	var result APIResult

	result.companyNumber = mapped[lCompanyNumber]
	result.registrationNumber = mapped[lRegistrationNumber]

	result.address.city = mapped[lCity]
	result.address.country = mapped[lCountry]
	result.address.postalCode = mapped[lPostalCode]
	result.address.department = mapped[lDepartment]

	result.legalEntityType = mapLegalEntityType(mapped[lLegalEntityType])

	if result.legalEntityType == legalEntityTypeCompany {
		result.company.name = mapped[lCompanyName]
		result.company.acronym = mapped[lCompanyName]
		result.company.brand = mapped[lCompanyName]
		result.company.contact.firstName = mapped[lContactFirstName]
		result.company.contact.lastName = mapped[lContactLastName]
		result.company.companyType = mapCompanyType(mapped[lCompanyType])
		result.company.brand = mapped[lBrand]
		result.company.acronym = mapped[lAcronym]

	} else if result.legalEntityType == legalEntityTypeIndividual {
		result.individual.title = mapPersonTitle(mapped[lIndividualTitle])
		result.individual.name.firstName = mapped[lIndividualFirstName]
		result.individual.name.lastName = mapped[lIndividualLastName]
	}

	fmt.Println(mapped[lExpirationDate])

	result.expirationDate, _ = time.Parse(
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
		"rechercheCriteres.numeroInscription": {params[sRegistrationNumber]},
		"rechercheCriteres.nomRepresentantLegal": {""},
		"rechercheCriteres.nomDenomination": {""},
		"rechercheCriteres.numeroSiren": {params[sCompanyNumber]},
		"rechercheCriteres.sigle": {""},
		"rechercheCriteres.marque": {""},
		"rechercheCriteres.autreFormeJuridique": {""},
		"rechercheCriteres.idFormeJuridique": {""},
		"rechercheCriteres.ville": {""},
		"rechercheCriteres.idPays": {""},
		"rechercheCriteres.codePostal": {""},
		"rechercheCriteres.idRegion": {""},
		"rechercheCriteres.idDepartement": {""},
		"action:/public/rechercheExploitant.liste.avancee": {"Rechercher"},
	})
	
	if err != nil {
		return APIResult{}, err
	}

	return handleSingleResultPage(resp)
}


func GetByCompanyNumber(companyNumber string) (APIResult, error) {
	return GetByAdvancedSearch(map[APISearchParams]string {
		sCompanyNumber: companyNumber,
	})
}


func GetByRegistrationNumber(registrationNumber string) (APIResult, error) {
	return GetByAdvancedSearch(map[APISearchParams]string {
		sRegistrationNumber: registrationNumber,
	})
}

func main() {
	// res, err := GetByRecordId(12345)

	res, err := GetByRegistrationNumber("EVTC123456789")

	if err != nil {
		fmt.Printf("Unable to fetch record")
		return
	}

	fmt.Println(res.companyNumber)
	fmt.Println(res.expirationDate)
}
