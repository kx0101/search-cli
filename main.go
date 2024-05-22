package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func searchFiles(root, filename string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.Contains(info.Name(), filename) {
			results <- path
		}

		return nil
	})

	if err != nil {
		results <- fmt.Sprintf("Error: %v", err)
	}
}

func main() {
	app := tview.NewApplication()
	inputField := tview.NewInputField().
		SetLabel("Αναζήτηση: ").
		SetFieldWidth(30)

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(inputField, 1, 0, true).
		AddItem(textView, 0, 1, false)

	var mu sync.Mutex

	var results []string

	resultsChan := make(chan string)
	var wg sync.WaitGroup

	go func() {
		for result := range resultsChan {
			mu.Lock()

			results = append(results, result)

			app.QueueUpdateDraw(func() {
				fmt.Fprintf(textView, "%s\n", result)
			})

			mu.Unlock()
		}
	}()

	selectedIndex := -1

	inputField.SetChangedFunc(func(text string) {
		textView.Clear()

		results = nil
		selectedIndex = -1

		wg.Add(1)
		go searchFiles(".", text, resultsChan, &wg)
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if selectedIndex > 0 {
				selectedIndex--
				updateTextViewSelection(textView, selectedIndex, results)
			}
		case tcell.KeyDown:
			if selectedIndex < len(results)-1 {
				selectedIndex++
				updateTextViewSelection(textView, selectedIndex, results)
			}
		}

		return event
	})

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && selectedIndex >= 0 && selectedIndex < len(results) {
			text := inputField.GetText()
			if len(text) == 0 && selectedIndex < 0 {
				return
			}

			selectedPath := results[selectedIndex]

			if err := openFile(selectedPath); err != nil {
				textView.SetText(fmt.Sprintf("Error opening file: %v", err))
			}

			inputField.SetText("")
		}
	})

	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}

	wg.Wait()
	close(resultsChan)
}

func openFile(selectedPath string) error {
	fullPath := filepath.Join(".", selectedPath)

	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", fullPath).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", fullPath).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func updateTextViewSelection(textView *tview.TextView, selectedIndex int, results []string) {
	var newText strings.Builder

	for i, result := range results {
		if i == selectedIndex {
			newText.WriteString("[")
			fmt.Fprintf(&newText, "[green]%s[-]", result)
			newText.WriteString("] <-----------\n")
		} else {
			fmt.Fprintf(&newText, "%s\n", result)
		}
	}

	textView.SetText(newText.String())
}
