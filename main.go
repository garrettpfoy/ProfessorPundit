package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
  "sort"
  "regexp"
)

type Teacher struct {
  Typename string `json:"__typename"`
  ID string `json:"id"`
  LastName string `json:"lastName"`
  NumRatings int `json:"numRatings"`
  Ratings struct {
      Edges []struct {
          Cursor string `json:"cursor"`
          Node struct {
              Typename string `json:"__typename"`
              ClarityRating int `json:"clarityRating"`
              ClarityRatingRounded int `json:"clarityRatingRounded"`
              Class string `json:"class"`
              Comment string `json:"comment"`
              CreatedByUser bool `json:"createdByUser"`
              Date string `json:"date"`
              DifficultyRatingRounded int `json:"difficultyRatingRounded"`
              FlagStatus string `json:"flagStatus"`
              Grade string `json:"grade"`
              HelpfulRating int `json:"helpfulRating"`
              HelpfulRatingRounded int `json:"helpfulRatingRounded"`
              IWouldTakeAgain *string `json:"iWouldTakeAgain"`
              ID string `json:"id"`
              IsForOnlineClass bool `json:"isForOnlineClass"`
              LegacyID int `json:"legacyId"`
              TeacherNote *string `json:"teacherNote"`
          } `json:"node"`
      } `json:"edges"`
      PageInfo struct {
          EndCursor string `json:"endCursor"`
          HasNextPage bool `json:"hasNextPage"`
      } `json:"pageInfo"`
  } `json:"ratings"`
}

type Data struct {
  Node Teacher `json:"node"`
}

type Root struct {
  Data Data `json:"data"`
}

type TeacherData struct {
	AvgDifficulty            float64 `json:"avgDifficulty"`
	AvgRatingRounded         float64 `json:"avgRatingRounded"`
	FirstName                string  `json:"firstName"`
	ID                       string  `json:"id"`
	LastName                 string  `json:"lastName"`
	NumRatings               int     `json:"numRatings"`
	WouldTakeAgainPercentRounded float64  `json:"wouldTakeAgainPercentRounded"`
}

type Edge struct {
	TeacherData TeacherData `json:"node"`
}

type Filters struct {
	Field   string `json:"field"`
	Options []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	} `json:"options"`
}

type Teachers struct {
	Edges []Edge `json:"edges"`
	Filters []Filters `json:"filters"`
	ResultCount int `json:"resultCount"`
}

type Search struct {
	Teachers Teachers `json:"teachers"`
}

type TeacherResponse struct {
	Data struct {
		School struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"school"`
		Search Search `json:"search"`
	} `json:"data"`
}

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

func handleError(err error) {
  if(err != nil) {
    panic(err)
  }
}

func getAllReviewsByTeacher(teacherID string, count int) ReviewResponse {
  graphql := GraphQL {
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
    Variables: map[string]interface{} {
      "count": count,
      "id": teacherID,
      "cursor": ""
    },
  }
}

func getAllTeachersByDepartment(departmentID string) TeacherResponse {
  //Hacky version of what RateMyProfs backend uses when loading more teachers when you click "View All Professors"
  graphql := GraphQL {
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
    Variables: map[string]interface{} {
      "query": map[string]interface{} {
        "text": "",
        "schoolID": "U2Nob29sLTExMTE=",
        "fallback": "false",
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

  var data TeacherResponse

  if err := json.Unmarshal(response, &data); err != nil {
    panic(err)
  } else {
    return data
  }
}

func printResponse(response Response) {
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

func main() {

  data := getAllTeachersByDepartment("RGVwYXJ0bWVudC0xMQ==")

  printResponse(data)

}