package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sort"
	"time"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SchoolID string `yaml:"schoolID"`
	Departments []string `yaml:"departmentID"`
}

//Struct that takes all GraphQL structs and combines them into something useful!
//This sorts all professor data by CLASS rather then department or instructor.
//Useful for when selecting a CLASS you can see the top professors for that class.
type Class struct {
    ClassName     string
    NumProfessors int
    NumReviews    int
    Professors    []Professor
}

type Professor struct {
    ProfessorName     string
    NumReviews        int
    AvgGrade          string
    AvgRating         float64
    AvgWouldTakeAgain float64
    TopReview         string
}

//"Final" struct that holds all teacher reviews and information
//fetched from #getAllReviewsByTeacher
type TeacherReviews struct {
	Typename   string `json:"__typename"`
	ID         string `json:"id"`
	LastName   string `json:"lastName"`
	NumRatings int    `json:"numRatings"`
	Ratings    struct {
		Edges []struct {
			Cursor string `json:"cursor"`
			Node   struct {
				Typename                string  `json:"__typename"`
				ClarityRating           int     `json:"clarityRating"`
				ClarityRatingRounded    int     `json:"clarityRatingRounded"`
				Class                   string  `json:"class"`
				Comment                 string  `json:"comment"`
				CreatedByUser           bool    `json:"createdByUser"`
				Date                    string  `json:"date"`
				DifficultyRatingRounded int     `json:"difficultyRatingRounded"`
				FlagStatus              string  `json:"flagStatus"`
				Grade                   string  `json:"grade"`
				HelpfulRating           int     `json:"helpfulRating"`
				HelpfulRatingRounded    int     `json:"helpfulRatingRounded"`
				IWouldTakeAgain         bool `json:"iWouldTakeAgain"`
				ID                      string  `json:"id"`
				IsForOnlineClass        bool    `json:"isForOnlineClass"`
				LegacyID                int     `json:"legacyId"`
			} `json:"node"`
		} `json:"edges"`
		PageInfo struct {
			EndCursor   string `json:"endCursor"`
			HasNextPage bool   `json:"hasNextPage"`
		} `json:"pageInfo"`
	} `json:"ratings"`
}

//Helper struct to contain TeacherReviews to match schema returned in JSON
//from the GraphQL request.
type Data struct {
	TeacherReviews TeacherReviews `json:"node"`
}

//Struct to contain the schema returned in JSON from the
//getAllTeacherReviews's GraphQL POST request.
type TeacherReviewsResponse struct {
	Data Data `json:"data"`
}

//"Final" struct that holds teacher infomation, most useful is the ID so we can fetch
//all of the professor's reviews, but other information is included so we don't have
//to calculate averages on our owns (or sum of ratings)
type TeacherData struct {
	AvgDifficulty                float64 `json:"avgDifficulty"`
	AvgRatingRounded             float64 `json:"avgRatingRounded"`
	FirstName                    string  `json:"firstName"`
	ID                           string  `json:"id"`
	LastName                     string  `json:"lastName"`
	NumRatings                   int     `json:"numRatings"`
	WouldTakeAgainPercentRounded float64 `json:"wouldTakeAgainPercentRounded"`
}

//Helper struct to hold the "Edge", note a lot of these structs are super messy because
//I am mimicing the GraphQL requests from Chrome's developer tools, there is no public schema
//for RateMyProfessors at the moment (without fetching through Insomnia or another tool)
type Edge struct {
	TeacherData TeacherData `json:"node"`
}

//Struct to hold the filters used in the GraphQL query for all professors.
//Doesn't really need to be included, but decided to include it in case anyone
//wants to make a PR to make this interface a bit more robust.
type Filters struct {
	Field   string `json:"field"`
	Options []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	} `json:"options"`
}

//Helper struct that holds Edges, which contains TeacherData instances.
//It also gives us a nice result count, which I have found to be inaccurate
//(unless it is describing something other than the edges count + filters count)
type Teachers struct {
	Edges       []Edge    `json:"edges"`
	Filters     []Filters `json:"filters"`
	ResultCount int       `json:"resultCount"`
}

//Search object is the result of the filter, it is also what leads us to
//indidual TeacherData
type Search struct {
	Teachers Teachers `json:"teachers"`
}

//TeachersResponse is the struct used to take the JSON response from
//the GraphQL POST request recieved in the getAllTeachersByDepartment()
//function.
type TeachersResponse struct {
	Data struct {
		School struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"school"`
		Search Search `json:"search"`
	} `json:"data"`
}

//Struct to hold a generic GraphQL query and variables. Used in all GraphQL POST
//requests that are queries, and not mutations.
type GraphQL struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

//Helper function to reduce line complexity of the file. Simply panics at any non-null errors.
func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

//Function to load in a config file given a string filename
func loadConfig(fileName string) Config {
	var config Config

	ymlFile, err := ioutil.ReadFile(fileName)
	handleError(err)

	err = yaml.Unmarshal(ymlFile, &config)
	handleError(err)

	return config
}

//Function to take a given date (2004-12-06 00:33:51 +0000 UTC type), and determine
//if it is older than @months old.
func isReviewValid(date string, months int) bool {
    t, err := time.ParseInLocation("2006-01-02 15:04:05 -0700 UTC", date, time.UTC)
	handleError(err)

    // get the current time
    now := time.Now()

    // subtract the specified number of months from the current time
    limit := now.AddDate(0, -months, 0)

    // check if the parsed time is older than the limit
    return t.Before(limit)
}

func isClassValid(className string) bool {
	match, _ := regexp.MatchString(`^\D+-\d+$`, className)

    return match
}

//Function to take a ugly RateMyProf class like MATH123 and turn it into MATH-123 :D
//uses regex bleh
func formatClassName(className string) string {
    if className == "" {
        return ""
    }
    //compile the regular expression to match letters and numbers
    re := regexp.MustCompile("([a-zA-Z]+)([0-9]+)")
    //replace matched letters and numbers with the matched letters, "-", and matched numbers
    className = re.ReplaceAllString(className, "$1-$2")
    return className
}

//Function to get all reviews based on a teacherID. Note: This is not dependant
//on Department ID or anything, and can be used without any previous calls.
func getAllReviewsByTeacher(teacherID string, count int) TeacherReviewsResponse {
	graphql := GraphQL{
		Query: `
    query RatingsListQuery(
      $count: Int!
      $id: ID!
      $cursor: String
    ) {
      node(id: $id) {
        __typename
        ... on Teacher {
          ...RatingsList_teacher_4pguUW
        }
        id
      }
    }
    
    fragment RatingsList_teacher_4pguUW on Teacher {
      id
      lastName
      numRatings
      ratings(first: $count, after: $cursor) {
        edges {
          cursor
          node {
            ...Rating_rating
            id
            __typename
          }
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
    
    
    fragment Rating_rating on Rating {
      comment
      flagStatus
      createdByUser
      teacherNote {
        id
      }
      ...RatingHeader_rating
      ...RatingValues_rating
      ...CourseMeta_rating
      ...RatingFooter_rating
      ...ProfessorNoteSection_rating
    }
    
    fragment RatingHeader_rating on Rating {
      date
      class
      helpfulRating
      clarityRating
      isForOnlineClass
    }
    
    
    fragment RatingValues_rating on Rating {
      helpfulRatingRounded
      clarityRatingRounded
      difficultyRatingRounded
    }
    
    fragment CourseMeta_rating on Rating {
      iWouldTakeAgain
      grade
    }
    
    
    fragment RatingFooter_rating on Rating {
      id
      comment
      teacherNote {
        id
      }
    }
    
    fragment ProfessorNoteSection_rating on Rating {
      teacherNote {
        ...ProfessorNote_note
        id
      }
      ...ProfessorNoteEditor_rating
    }
    
    fragment ProfessorNote_note on TeacherNotes {
      comment
      ...ProfessorNoteHeader_note
    }
    
    fragment ProfessorNoteEditor_rating on Rating {
      id
      legacyId
      class
      teacherNote {
        id
        teacherId
        comment
      }
    }
    
    fragment ProfessorNoteHeader_note on TeacherNotes {
      createdAt
      updatedAt
    }
    `,
		Variables: map[string]interface{}{
			"count":  count,
			"id":     teacherID,
			"cursor": "",
		},
	}

	query, err := json.Marshal(graphql)
	handleError(err)

	req, err := http.NewRequest("POST", "https://www.ratemyprofessors.com/graphql", bytes.NewBuffer(query))
	handleError(err)
	req.Header.Add("Authorization", "Basic dGVzdDp0ZXN0")

	client := &http.Client{}

	res, err := client.Do(req.WithContext(context.Background()))
	handleError(err)
	defer res.Body.Close()

	response, err := ioutil.ReadAll(res.Body)
	handleError(err)

	var data TeacherReviewsResponse

	if err := json.Unmarshal(response, &data); err != nil {
		panic(err)
	} else {
		return data
	}

}

//Function to get all teachers in a given department (from ID). Read the README for
//instructions on how to get department IDS
func getAllTeachersByDepartment(departmentID string) TeachersResponse {
	//Hacky version of what RateMyProfs backend uses when loading more teachers when you click "View All Professors"
	graphql := GraphQL{
		Query: `
    query TeacherSearchResultsPageQuery(
      $query: TeacherSearchQuery!
      $schoolID: ID
    ) {
      search: newSearch {
        ...TeacherSearchPagination_search_1ZLmLD
      }
      school: node(id: $schoolID) {
        ... on School {
          name
        }
        id
      }
    }
    
    fragment TeacherSearchPagination_search_1ZLmLD on newSearch {
      teachers(query: $query, first: 500, after: "") {
        edges {
          node {
            ...TeacherCard_teacher
            id
          }
        }
        resultCount
        filters {
          field
          options {
            value
            id
          }
        }
      }
    }
    
    fragment TeacherCard_teacher on Teacher {
      id
      avgRatingRounded
      numRatings
      ...CardFeedback_teacher
      ...CardName_teacher
    }
    
    fragment CardFeedback_teacher on Teacher {
      wouldTakeAgainPercentRounded
      avgDifficulty
    }
    
    fragment CardName_teacher on Teacher {
      firstName
      lastName
    }
    `,
		Variables: map[string]interface{}{
			"query": map[string]interface{}{
				"text":         "",
				"schoolID":     "U2Nob29sLTExMTE=",
				"fallback":     "false",
				"departmentID": departmentID,
			},
			"schoolID": "RGVwYXJ0bWVudC0xMQ==",
		},
	}

	query, err := json.Marshal(graphql)
	handleError(err)

	req, err := http.NewRequest("POST", "https://www.ratemyprofessors.com/graphql", bytes.NewBuffer(query))
	handleError(err)
	req.Header.Add("Authorization", "Basic dGVzdDp0ZXN0")

	client := &http.Client{}

	res, err := client.Do(req.WithContext(context.Background()))
	handleError(err)
	defer res.Body.Close()

	response, err := ioutil.ReadAll(res.Body)
	handleError(err)

	var data TeachersResponse

	if err := json.Unmarshal(response, &data); err != nil {
		panic(err)
	} else {
		return data
	}
}

// Function to take a Teachers response, iterate through each teacher in the department,
// fetch all reviews, then sort them into classes accordingly.
func generateClasses(response TeachersResponse) map[string]Class {

	classes := make(map[string]Class)

	for _, edge := range response.Data.Search.Teachers.Edges {
		teacherID := edge.TeacherData.ID

		//Now we have the teacher ID, we gotta now fetch all of the reviews done so we can compile
		//a map of class-reviews. 
		teacherReviewsResponse := getAllReviewsByTeacher(teacherID, 200)

		//We also want to define our Professor struct so if they teach a class, we can just add their
		//information to the Class object.
		professor := Professor{
			ProfessorName: edge.TeacherData.FirstName + edge.TeacherData.LastName,
			NumReviews: 0,
			AvgGrade: "N/A",
			AvgRating: edge.TeacherData.AvgRatingRounded,
			AvgWouldTakeAgain: edge.TeacherData.WouldTakeAgainPercentRounded,
			TopReview: "N/A",
		}

		//Now we have a response with all reviews, let's sort this into the map :D
		for _, edge := range teacherReviewsResponse.Data.TeacherReviews.Ratings.Edges {
			//Increment numReviews
			professor.NumReviews = professor.NumReviews + 1
			//Validity checks for review
			if(isReviewValid(edge.Node.Date, 24) || !isClassValid(formatClassName(edge.Node.Class))) {
				continue
			}

			className := formatClassName(edge.Node.Class)

			//First check to see if we have the class in the map already
			_, ok := classes[className]

			if !ok {
				//Not in map already
				class := Class{
					ClassName: className,
					NumProfessors: 1,
					NumReviews: 1,
					Professors: []Professor{professor},
				}

				classes[className] = class
			} else {
				class := classes[className]

				class.NumReviews = class.NumReviews + 1

				//Now check if is already in the professors
				isIn := false

				for _, prof := range class.Professors {
					if(prof.ProfessorName == professor.ProfessorName) {
						isIn = true
					}
				}

				if(!isIn) {
					//Add professor to class
					class.Professors = append(class.Professors, professor)
					class.NumProfessors = class.NumProfessors + 1
				} 

				classes[className] = class
			}
		}
	}

	return classes
}

func printClasses(classes map[string]Class) {
	for _, class := range classes {
		fmt.Println(class)
	}
}

//Helper function to help display a TeachersResponse.
//Printed in order from HIGHEST rating to LOWEST rating.
func printTeachers(response TeachersResponse) {
	//First sort by highest average rating
	sort.Slice(response.Data.Search.Teachers.Edges, func(i, j int) bool {
		return response.Data.Search.Teachers.Edges[i].TeacherData.AvgRatingRounded > response.Data.Search.Teachers.Edges[j].TeacherData.AvgRatingRounded
	})

	for _, edge := range response.Data.Search.Teachers.Edges {
		teacher := edge.TeacherData
		fmt.Println("\n----------------------------------------")
		fmt.Println("Teacher Name:", teacher.FirstName, teacher.LastName)
		fmt.Println("Average Difficulty:", teacher.AvgDifficulty)
		fmt.Println("Average Rating:", teacher.AvgRatingRounded)
		fmt.Println("Percent who would take again:", teacher.WouldTakeAgainPercentRounded, "%")
		fmt.Println("----------------------------------------")
	}
}

//Helper function to print a TeacherReviewsResponse. Not ordered at the moment.
func printReviews(response TeacherReviewsResponse) {

	for _, edge := range response.Data.TeacherReviews.Ratings.Edges {
		// if(isReviewValid(edge.Node.Date, 1)) {
		// 	continue
		// }
		// if(!isClassValid(formatClassName(edge.Node.Class))) {
		// 	continue
		// }

		fmt.Println("\n----------------------------------------")
		fmt.Println("Review:")
		fmt.Println("  Class: ", formatClassName(edge.Node.Class))
		fmt.Println("  Review: ", edge.Node.Comment)
		fmt.Println("  Grade:  ", edge.Node.Grade)
		fmt.Println("  Date: ", edge.Node.Date)
		fmt.Println("----------------------------------------")
	}
}

func main() {

	config := loadConfig("config.yml")

	fmt.Println("School ID is: ", config.SchoolID)

	// data := getAllTeachersByDepartment("RGVwYXJ0bWVudC00Ng==")

	// printTeachers(data)

	// // data2 := getAllReviewsByTeacher("VGVhY2hlci0yNjUyNDU3", 100)

	// // printReviews(data2)

	// classes := generateClasses(data)

	// printClasses(classes)
}
