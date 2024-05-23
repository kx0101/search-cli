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

func searchFiles(root, filename string, resultsChan chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.Contains(info.Name(), filename) {
			resultsChan <- path
		}

		return nil
	})

	if err != nil {
		resultsChan <- fmt.Sprintf("Error: %v", err)
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

	seen := make(map[string]struct{})
	var results []string

	resultsChan := make(chan string)
	var wg sync.WaitGroup

	go func() {
		for result := range resultsChan {
			mu.Lock()

			if _, exists := seen[result]; !exists {
				seen[result] = struct{}{}
				results = append(results, result)

				app.QueueUpdateDraw(func() {
					fmt.Fprintf(textView, "%s\n", result)
				})
			}

			mu.Unlock()
		}
	}()

	selectedIndex := -1

	inputField.SetChangedFunc(func(text string) {
		textView.Clear()

		mu.Lock()
		seen = make(map[string]struct{})
		results = nil
		selectedIndex = -1
		mu.Unlock()

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
			if selectedIndex < len(seen)-1 {
				selectedIndex++
				updateTextViewSelection(textView, selectedIndex, results)
			}
		}

		return event
	})

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter && selectedIndex >= 0 && selectedIndex < len(seen) {
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

func updateTextViewSelection(textView *tview.TextView, selectedIndex int, seen []string) {
	var newText strings.Builder

	for i, result := range seen {
		if i == selectedIndex {
			newText.WriteString("[")
			fmt.Fprintf(&newText, "[green]%s[-]", result)
			newText.WriteString("] <-----------\n")
		} else {
			fmt.Fprintf(&newText, "%s\n", result)
		}
	}

	textView.SetText(newText.String())
	moveScreenToHighlightedCentered(textView, selectedIndex)
}

func moveScreenToHighlightedCentered(textView *tview.TextView, selectedIndex int) {
	_, _, _, height := textView.GetRect()
	if height > 0 {
		centerOffset := selectedIndex - height/2
		if centerOffset < 0 {
			centerOffset = 0
		}

		textView.ScrollTo(centerOffset, 0)
	}
}
