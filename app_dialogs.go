package main

import "github.com/wailsapp/wails/v3/pkg/application"

// fileFilter is a file dialog filter, e.g. {"Imagens", "*.png;*.jpg"}.
type fileFilter struct{ name, pattern string }

func (a *App) openDialog(title string, filters []fileFilter) *application.OpenFileDialogStruct {
	d := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(true)
	for _, f := range filters {
		d.AddFilter(f.name, f.pattern)
	}
	if a.window != nil {
		d.AttachToWindow(a.window)
	}
	return d
}

// openFile asks for one file; returns "" when cancelled.
func (a *App) openFile(title string, filters ...fileFilter) (string, error) {
	return a.openDialog(title, filters).PromptForSingleSelection()
}

// openFiles asks for one or more files.
func (a *App) openFiles(title string, filters ...fileFilter) ([]string, error) {
	return a.openDialog(title, filters).PromptForMultipleSelection()
}

// openDirectory asks for a folder, starting at dir.
func (a *App) openDirectory(title, dir string) (string, error) {
	d := application.Get().Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).
		CanCreateDirectories(true).SetDirectory(dir)
	if a.window != nil {
		d.AttachToWindow(a.window)
	}
	return d.PromptForSingleSelection()
}

// saveFile asks where to save; returns "" when cancelled.
func (a *App) saveFile(title, filename string) (string, error) {
	d := application.Get().Dialog.SaveFile().SetMessage(title).SetFilename(filename).CanCreateDirectories(true)
	if a.window != nil {
		d.AttachToWindow(a.window)
	}
	return d.PromptForSingleSelection()
}
