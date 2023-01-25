/*
	Copyright <2022> Nik Ogura <nik.ogura@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
package boilerplate

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/spf13/afero"
	"io"
	"path/filepath"
	"strings"
	"text/template"
)

type FilePath struct {
	Path      string
	Name      string
	TemplPath string
	TemplName string
	IsDir     bool
}

type TmplWriter struct {
	OutFs     afero.Fs
	TemplFs   embed.FS
	FilePaths []FilePath
	ProjDir   string
	TmplVals  map[string]interface{}
}

func NewTmplWriter(outFs afero.Fs, projType string, vals map[string]interface{}) (TmplWriter, error) {
	fs, dirName, err := GetProjectFs(projType)
	if err != nil {
		return TmplWriter{}, fmt.Errorf("fs error: %v", err)
	}

	w := TmplWriter{
		OutFs:    outFs,
		TemplFs:  fs,
		ProjDir:  dirName,
		TmplVals: vals}

	if w.FilePaths, err = w.GetFilePaths(dirName); err != nil {
		return w, fmt.Errorf("failed to walk filepath from root(%s): %v", ".", err)
	}
	return w, nil
}

func (w TmplWriter) BuildProject(destDir string) error {
	//fp := w.FilePaths
	if err := w.ResolveAllPathTemplates(); err != nil {
		return err
	}

	w.fixGoModTemplPaths()

	if err := w.CreateAllFilePathsAtRoot(destDir); err != nil {
		return err
	}

	if err := w.WriteAllDestFileTemplateData(destDir); err != nil {
		return err
	}

	return nil
}

func (w TmplWriter) ResolveAllPathTemplates() error {
	for i := range w.FilePaths {
		fp := w.FilePaths[i]
		if buf, err := w.ResolveTemplateVars(fp.Path); err != nil {
			return fmt.Errorf("path resolution failure: path=%s, err=%v", fp.Path, err)
		} else {
			path := strings.Replace(buf.String(), w.ProjDir, "", 1)
			if path[0] == '/' {
				path = path[1:]
			}
			w.FilePaths[i].TemplPath = path
		}

		if buf, err := w.ResolveTemplateVars(fp.Name); err != nil {
			return fmt.Errorf("name resolution failure: path=%s, err=%v", fp.Name, err)
		} else {
			name := strings.Replace(buf.String(), w.ProjDir, "", 1)
			if name[0] == '/' {
				name = name[1:]
			}
			w.FilePaths[i].TemplName = name
		}
	}

	return nil
}

func (w TmplWriter) ResolveTemplateVars(str string) (*bytes.Buffer, error) {
	tmpl, err := template.New("tmplWriter").Parse(str)
	if err != nil {
		return nil, fmt.Errorf("path parsing error: %v", err)
	}

	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, w.TmplVals)
	if err != nil {
		return nil, fmt.Errorf("failed to exec template from string(%s): %v", str, err)
	}

	return buf, nil
}

func (w TmplWriter) CreateAllFilePathsAtRoot(root string) error {
	for _, fp := range w.FilePaths {
		if err := w.CreatePath(root, fp.TemplPath); err != nil {
			return fmt.Errorf("failed to create file(%s): err(%s)", fp.TemplPath, err)
		}
	}
	return nil
}

func (w TmplWriter) CreatePath(root, file string) error {
	path := fmt.Sprintf("%s/%s", root, file)
	if err := w.OutFs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("directory creation failed: %v", err)
	}
	return nil
}

func (w TmplWriter) WriteAllDestFileTemplateData(destDir string) error {
	for _, fp := range w.FilePaths {
		if fp.IsDir {
			continue
		}

		if err := w.WriteFileTemplateData(fp, destDir); err != nil {
			return fmt.Errorf("writing file(%s) failed: %v", fp.TemplName, err)
		}
	}

	return nil
}

func (w TmplWriter) WriteFileTemplateData(fp FilePath, destDir string) error {
	path := fmt.Sprintf("%s/%s", destDir, fp.TemplPath)
	file, err := w.OutFs.Open(path)
	if err != nil {
		file, err = w.OutFs.Create(path)
		if err != nil {
			return fmt.Errorf("failed to open file(%s): %v", path, err)
		}
	}
	defer file.Close()

	buf, err := w.ResolveFileTemplateData(fp)
	if err != nil {
		return fmt.Errorf("failed to exec template: %v", err)
	}

	if buf, err = w.removeBuildExclusions(buf); err != nil {
		return fmt.Errorf("failed to remove build exclusions from file(%s): %v", fp.TemplName, err)
	}

	n, err := file.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("cannot write file(%s) bytes: %v", path, err)
	} else if n != buf.Len() {
		return fmt.Errorf("wrong number of bytes written: exp(%d) act(%d)", buf.Len(), n)
	}

	return nil
}

func (w TmplWriter) ResolveFileTemplateData(fp FilePath) (*bytes.Buffer, error) {
	// Read the original file data not the parsed template path
	file, err := w.TemplFs.Open(fp.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot open file(%s): err(%v)", fp.Path, err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file data(%s): err(%s)", fp.Path, err)
	}

	buf, err := w.ResolveTemplateVars(string(data))
	if err != nil {
		return nil, fmt.Errorf("cannot execute template with file(%s): %v", fp.Path, err)
	}

	return buf, nil
}

func (w TmplWriter) GetFilePaths(root string) ([]FilePath, error) {
	var fp []FilePath

	if entries, err := w.TemplFs.ReadDir(root); err != nil {
		return fp, fmt.Errorf("failed to read embedded files at dir(%s): %v", root, err)
	} else {
		for _, e := range entries {
			cpath := fmt.Sprintf("%s/%s", root, e.Name())
			if e.IsDir() {
				children, err := w.GetFilePaths(cpath)
				if err != nil {
					return nil, fmt.Errorf("failed to collected children at root path(%s): %v", cpath, err)
				}
				fp = append(fp, children...)
			} else {
				fp = append(fp, FilePath{
					Path: cpath,
					Name: e.Name(),
				})
			}
		}
	}

	return fp, nil
}

func (w TmplWriter) fixGoModTemplPaths() {
	for i := range w.FilePaths {
		fp := w.FilePaths[i]
		if fp.TemplName == "go.mod_" || fp.TemplName == "go.sum_" {
			fp.TemplPath = fp.TemplPath[:len(fp.TemplPath)-1]
			fp.TemplName = fp.TemplName[:len(fp.TemplName)-1]
		}
		w.FilePaths[i] = fp
	}
}

func (w TmplWriter) removeBuildExclusions(buf *bytes.Buffer) (*bytes.Buffer, error) {
	replBuf, err := w.ResolveTemplateVars("// +build exclude {{.ProjectName}}\n")
	if err != nil {
		return nil, fmt.Errorf("removing build exclusions cannot execute template: %v", err)
	}

	return bytes.NewBufferString(strings.ReplaceAll(buf.String(), replBuf.String(), "")), nil
}
