package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- ECR ---

func (a App) openECRRepos() (App, tea.Cmd) {
	a.mode = modeECR
	a.state = viewECRRepos
	a.ecrReposView = views.NewECRRepos()
	a.ecrReposView = a.ecrReposView.SetSize(a.width-3, a.height-6)
	a.loading = true
	ecrService, ctx := a.ecr, a.ctx
	return a, func() tea.Msg {
		repos, err := ecrService.ListRepositories(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ecrReposLoadedMsg{repos}
	}
}

func (a App) openECRImages(repoName, repoURI string) (App, tea.Cmd) {
	a.state = viewECRImages
	a.ecrImagesView = views.NewECRImages(repoName, repoURI)
	a.ecrImagesView = a.ecrImagesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	ecrService, ctx := a.ecr, a.ctx
	return a, func() tea.Msg {
		images, err := ecrService.ListImages(ctx, repoName)
		if err != nil {
			return errMsg{err}
		}
		return ecrImagesLoadedMsg{images}
	}
}

func (a App) openECRFindings() (App, tea.Cmd) {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return a, nil
	}
	repoName := a.ecrImagesView.RepoName()
	a.state = viewECRFindings
	a.ecrFindingsView = views.NewECRFindings(repoName, img.Digest, img.Tags)
	a.ecrFindingsView = a.ecrFindingsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	ecrService, ctx := a.ecr, a.ctx
	digest := img.Digest
	return a, func() tea.Msg {
		scan, err := ecrService.ScanFindings(ctx, repoName, digest)
		if err != nil {
			return errMsg{err}
		}
		return ecrFindingsLoadedMsg{scan}
	}
}

func (a App) startECRScan() (App, tea.Cmd) {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return a, nil
	}
	repoName := a.ecrImagesView.RepoName()
	ecrService, ctx := a.ecr, a.ctx
	image := *img
	a.loading = true
	return a, func() tea.Msg {
		err := ecrService.StartScan(ctx, repoName, image)
		if err != nil {
			return errMsg{err}
		}
		tagLabel := image.Digest[:min(19, len(image.Digest))]
		if len(image.Tags) > 0 {
			tagLabel = image.Tags[0]
		}
		return ecrActionDoneMsg{fmt.Sprintf("Scan started for %s:%s", repoName, tagLabel)}
	}
}

func (a App) deleteECRImage() (App, tea.Cmd) {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return a, nil
	}
	tagLabel := img.Digest[:min(19, len(img.Digest))]
	if len(img.Tags) > 0 {
		tagLabel = strings.Join(img.Tags, ", ")
	}
	a.confirm = NewConfirm(ConfirmECRDelete,
		fmt.Sprintf("Delete image %s from %s?", tagLabel, a.ecrImagesView.RepoName()))
	return a, nil
}

func (a App) doDeleteECRImage() tea.Cmd {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return nil
	}
	ecrService, ctx := a.ecr, a.ctx
	repoName := a.ecrImagesView.RepoName()
	digest := img.Digest
	return func() tea.Msg {
		err := ecrService.DeleteImage(ctx, repoName, digest)
		if err != nil {
			return errMsg{err}
		}
		return ecrActionDoneMsg{fmt.Sprintf("Deleted image %s", digest[:min(19, len(digest))])}
	}
}

func (a App) copyECRImageURI() (App, tea.Cmd) {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return a, nil
	}
	repoURI := a.ecrImagesView.RepoURI()
	tag := ""
	if len(img.Tags) > 0 {
		tag = img.Tags[0]
	}
	image := *img
	if tag == "" {
		image.Tags = nil
	}
	uri, err := a.ecr.ImageURI(repoURI, image)
	if err != nil {
		a.err = err
		return a, nil
	}
	if err := clipboard.WriteAll(uri); err != nil {
		a.err = fmt.Errorf("clipboard: %w", err)
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Copied: %s", uri)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) refreshECRRepos() tea.Cmd {
	ecrService, ctx := a.ecr, a.ctx
	return func() tea.Msg {
		repos, err := ecrService.ListRepositories(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ecrReposLoadedMsg{repos}
	}
}

func (a App) refreshECRImages() tea.Cmd {
	repoName := a.ecrImagesView.RepoName()
	ecrService, ctx := a.ecr, a.ctx
	return func() tea.Msg {
		images, err := ecrService.ListImages(ctx, repoName)
		if err != nil {
			return errMsg{err}
		}
		return ecrImagesLoadedMsg{images}
	}
}

func (a App) handleECRAction(msg ecrActionDoneMsg) (App, tea.Cmd) {
	a.flashMessage = msg.message
	a.flashExpiry = time.Now().Add(5 * time.Second)
	a.loading = false
	if a.state == viewECRImages {
		return a, a.refreshECRImages()
	}
	return a, nil
}
