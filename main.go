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
)

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
				IWouldTakeAgain         *string `json:"iWouldTakeAgain"`
				ID                      string  `json:"id"`
				IsForOnlineClass        bool    `json:"isForOnlineClass"`
				LegacyID                int     `json:"legacyId"`
				TeacherNote             *string `json:"teacherNote"`
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

//Function to take a ugly RateMyProf class like MATH123 and turn it into MATH-123 :D
//uses regex bleh
func formatClassName(className string) string {
	re := regexp.MustCompile("(?P<prefix>[A-Za-z]+)(?P<number>[0-9]+)")

	return re.ReplaceAllString(className, "${prefix}-${number}")
}

//Helper function to reduce line complexity of the file. Simply panics at any non-null errors.
func handleError(err error) {
	if err != nil {
		panic(err)
	}
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
      teachers(query: $query, first: 100, after: "") {
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
		fmt.Println("Review:")
		fmt.Println("  Class: ", edge.Node.Class)
		fmt.Println("  Comment: ", formatClassName(edge.Node.Comment))
		fmt.Println("  Date: ", edge.Node.Date)
	}
}

func main() {

	data := getAllTeachersByDepartment("RGVwYXJ0bWVudC0xMQ==")

	printTeachers(data)

	data2 := getAllReviewsByTeacher("VGVhY2hlci0xMDUwOQ==", 100)

	printReviews(data2)

}
