package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"syscall"
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

var (
	dll                   *syscall.LazyDLL
	calculateFunction     *syscall.LazyProc
	getFunctionParameters *syscall.LazyProc
)

func init() {
	dll = syscall.NewLazyDLL("D:/I understand it now/functions.dll")
	calculateFunction = dll.NewProc("calculateFunction")
	getFunctionParameters = dll.NewProc("getFunctionParameters")
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

func getFunctionHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	var response map[string]interface{}

	// Здесь вы можете вызвать вашу DLL или использовать логику, чтобы вернуть параметры
	if name == "Volosov" {
		response = map[string]interface{}{
			"function": "0.01 * x ** 2 + 50 * cos(x)",
			"x_start":  -10 * math.Pi,
			"x_end":    10 * math.Pi,
			"step":     math.Pi / 4,
		}
	} else if name == "Vasiliev" {
		response = map[string]interface{}{
			"function": "(1 / x) * sin(x) * 50",
			"x_start":  -10,
			"x_end":    10,
			"step":     0.1,
		}
	} else if name == "Suryaninova" {
		response = map[string]interface{}{
			"function": "x * sin(x) * sin(1000000 * x)",
			"x_start":  -15,
			"x_end":    15,
			"step":     0.3,
		}
	} else {
		// Обработайте другие фамилии
		http.Error(w, "Функция не найдена", http.StatusNotFound)
		return
	}

	// Логирование параметров для проверки
	fmt.Printf("Параметры функции %s: %+v\n", name, response)

	// Отправка ответа клиенту
	json.NewEncoder(w).Encode(response)
}

func main() {
	/*var xStart, xEnd, step float64
	name := "Suryaninova"

	getFunctionParameters.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))), uintptr(unsafe.Pointer(&xStart)), uintptr(unsafe.Pointer(&xEnd)), uintptr(unsafe.Pointer(&step)))

	fmt.Printf("Параметры функции %s : Xstart = %f, Xend = %f, Step = %f\n", name, xStart, xEnd, step)
	result, _, _ := calculateFunction.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(name))), uintptr(5.0))
	fmt.Printf("Результат: %f\n", math.Float64frombits(uint64(result)))*/

	http.HandleFunc("/", indexHandler)

	http.HandleFunc("/fetch", fetchHandler)

	http.HandleFunc("/get_function", getFunctionHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	port := ":8080"
	log.Printf("Сервер запущен на http://localhost%s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}
