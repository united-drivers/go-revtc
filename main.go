package main

import (
	"errors"
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/gin-gonic/gin"
	google_protobuf "github.com/golang/protobuf/ptypes/timestamp"
	pb "github.com/united-drivers/go-revtc/proto"
	"golang.org/x/net/html"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseUrl = "https://registre-vtc.developpement-durable.gouv.fr/public"

type APISearchParams int

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

var personTitleMapping = []string{
	pb.PERSON_TITLE_PERSON_TITLE_MR:  "M.",
	pb.PERSON_TITLE_PERSON_TITLE_MRS: "Mme",
}

var businessEntityTypeMapping = []string{
	pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_SA:   "Société anonyme",
	pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_SARL: "Société à responsabilité limitée",
	pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_SAS:  "Société par actions simplifiée",
	pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_SASU: "Société par actions simplifiée unipersonnelle",
	pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_EURL: "Entreprise unipersonnelle à responsabilité limitée",
}

var legalEntityTypeMapping = []string{
	pb.LEGAL_ENTITY_TYPE_LEGAL_ENTITY_TYPE_COMPANY:    "Personne morale",
	pb.LEGAL_ENTITY_TYPE_LEGAL_ENTITY_TYPE_INDIVIDUAL: "Personne physique",
}

func getKeyForMappingValue(mapping []string, inputValue string, defaultValue int) int {
	for key, value := range mapping {
		if value == inputValue {
			return key
		}
	}

	return defaultValue
}

func castAPIPersonTitle(str string) pb.PERSON_TITLE {
	return pb.PERSON_TITLE(getKeyForMappingValue(personTitleMapping, str, int(pb.PERSON_TITLE_PERSON_TITLE_OTHER)))
}

func castAPIBusinessEntityType(str string) pb.BUSINESS_ENTITY_TYPE {
	return pb.BUSINESS_ENTITY_TYPE(getKeyForMappingValue(businessEntityTypeMapping, str, int(pb.BUSINESS_ENTITY_TYPE_BUSINESS_ENTITY_TYPE_OTHER)))
}

func castAPILegalEntityType(str string) pb.LEGAL_ENTITY_TYPE {
	return pb.LEGAL_ENTITY_TYPE(getKeyForMappingValue(legalEntityTypeMapping, str, int(pb.LEGAL_ENTITY_TYPE_LEGAL_ENTITY_TYPE_OTHER)))
}

func mapDictToObject(mapped map[string]string) pb.VTCEntry {
	var result = pb.VTCEntry{
		CompanyNumber:      mapped[lCompanyNumber],
		RegistrationNumber: mapped[lRegistrationNumber],
		LegalEntityType:    castAPILegalEntityType(mapped[lLegalEntityType]),
	}

	result.Address = &pb.Address{
		City:       mapped[lCity],
		Country:    mapped[lCountry],
		PostalCode: mapped[lPostalCode],
		Department: mapped[lDepartment],
	}

	if result.LegalEntityType == pb.LEGAL_ENTITY_TYPE_LEGAL_ENTITY_TYPE_COMPANY {
		result.Company = &pb.Company{
			Name:    mapped[lCompanyName],
			Acronym: mapped[lAcronym],
			Contact: &pb.PersonName{
				FirstName: mapped[lContactFirstName],
				LastName:  mapped[lContactLastName],
			},
			CompanyType: castAPIBusinessEntityType(mapped[lCompanyType]),
			Brand:       mapped[lBrand],
		}

	} else if result.LegalEntityType == pb.LEGAL_ENTITY_TYPE_LEGAL_ENTITY_TYPE_INDIVIDUAL {
		result.Individual = &pb.Individual{
			Title: castAPIPersonTitle(mapped[lIndividualTitle]),
			Name: &pb.PersonName{
				FirstName: mapped[lIndividualFirstName],
				LastName:  mapped[lIndividualLastName],
			},
		}
	}

	expirationDate, _ := time.Parse("02/01/2006", mapped[lExpirationDate])
	result.ExpirationDate = &google_protobuf.Timestamp{
		Seconds: int64(expirationDate.Second()),
		Nanos:   int32(expirationDate.Nanosecond()),
	}

	return result
}

func handleSingleResultPage(res *http.Response) (pb.VTCEntry, error) {
	if res.StatusCode != 200 {
		return pb.VTCEntry{}, errors.New("not found")
	}

	doc, errHtml := html.Parse(res.Body)
	sel, errCss := cascadia.Compile(".cLabel")

	if errHtml != nil {
		return pb.VTCEntry{}, errHtml
	}

	if errCss != nil {
		return pb.VTCEntry{}, errCss
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
		return pb.VTCEntry{}, errors.New("not found")
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

func GetByRecordId(recordId int) (pb.VTCEntry, error) {
	var requestUrl = fmt.Sprintf(
		"%s/rechercheExploitant.exploitantDetails.action?dossier.id=%d",
		baseUrl, recordId)

	resp, err := http.Get(requestUrl)

	if err != nil {
		return pb.VTCEntry{}, err
	}

	return handleSingleResultPage(resp)
}

func GetByAdvancedSearch(params map[APISearchParams]string) (pb.VTCEntry, error) {
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
		return pb.VTCEntry{}, err
	}

	return handleSingleResultPage(resp)
}

func GetByCompanyNumber(companyNumber string) (pb.VTCEntry, error) {
	return GetByAdvancedSearch(map[APISearchParams]string{
		sCompanyNumber: companyNumber,
	})
}

func GetByRegistrationNumber(registrationNumber string) (pb.VTCEntry, error) {
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
