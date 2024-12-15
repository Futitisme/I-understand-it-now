package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
)

type DataPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type DataPayload struct {
	Function string  `json:"function"`
	XStart   float64 `json:"x_start"`
	XEnd     float64 `json:"x_end"`
	Step     float64 `json:"step"`
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index.html", nil)
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	tmplPath := filepath.Join("templates", tmpl) //Собираем путь к шаблону
	t, err := template.ParseFiles(tmplPath)      //Загружаем HTML шаблон
	if err != nil {
		http.Error(w, "Ошибка загрузки шаблона", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data) //Если загрузка html шаблона произошла успешно, рендерим с переданной датой
}

func fetchData(url string, payload DataPayload) (string, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ошибка сериализации данных: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервер вернул ошибку: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении ответа: %v", err)
	}
	return string(body), nil
}

func fetchHandler(w http.ResponseWriter, r *http.Request) {
	apiUrl := "https://iunderstanditnow.pythonanywhere.com/calculate"

	if r.Method != http.MethodPost {
		http.Error(w, "Только POST-запросы разрешены", http.StatusMethodNotAllowed)
		return
	}

	var data DataPayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		http.Error(w, fmt.Sprintf("Ошибка при декодировании данных: %v", err), http.StatusBadRequest)
		return
	}

	totalPoints := int((data.XEnd-data.XStart)/data.Step) + 1
	if totalPoints <= 10 {
		resp, err := fetchData(apiUrl, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка в запросе: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}

	var allData []DataPoint
	currentStart := data.XStart

	for currentStart < data.XEnd {
		currentEnd := currentStart + float64(9)*data.Step
		if currentEnd > data.XEnd {
			currentEnd = data.XEnd
		}

		segmentPayload := DataPayload{
			Function: data.Function,
			XStart:   currentStart,
			XEnd:     currentEnd,
			Step:     data.Step,
		}

		resp, err := fetchData(apiUrl, segmentPayload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка в запросе: %v", err), http.StatusInternalServerError)
			return
		}

		var segmentData struct {
			Data []DataPoint `json:"data"`
		}
		if err := json.Unmarshal([]byte(resp), &segmentData); err != nil {
			http.Error(w, fmt.Sprintf("Ошибка при разборе данных: %v", err), http.StatusInternalServerError)
			return
		}

		allData = append(allData, segmentData.Data...)

		currentStart = currentEnd + data.Step
	}

	response := struct {
		Data []DataPoint `json:"data"`
	}{
		Data: allData,
	}
	resultJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка при сериализации данных: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(resultJSON))
}

func main() {
	http.HandleFunc("/", indexHandler)

	http.HandleFunc("/fetch", fetchHandler)

	port := ":8080"
	log.Printf("Сервер запущен на http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
