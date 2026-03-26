package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/atotto/clipboard"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- ECR ---

func (a App) openECRRepos() (App, tea.Cmd) {
	a.mode = modeECR
	a.state = viewECRRepos
	a.ecrReposView = views.NewECRRepos()
	a.ecrReposView = a.ecrReposView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	return a, func() tea.Msg {
		repos, err := client.ListECRRepos(context.Background(), "")
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
	client := a.client
	return a, func() tea.Msg {
		images, err := client.ListECRImages(context.Background(), repoName)
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
	if img.ScanStatus != "COMPLETE" {
		a.err = fmt.Errorf("no scan results available (status: %s) — press 's' to start a scan", img.ScanStatus)
		return a, nil
	}
	repoName := a.ecrImagesView.RepoName()
	a.state = viewECRFindings
	a.ecrFindingsView = views.NewECRFindings(repoName, img.Digest, img.Tags)
	a.ecrFindingsView = a.ecrFindingsView.SetSize(a.width-3, a.height-6)
	a.loading = true
	client := a.client
	digest := img.Digest
	return a, func() tea.Msg {
		findings, err := client.GetECRScanFindings(context.Background(), repoName, digest)
		if err != nil {
			return errMsg{err}
		}
		return ecrFindingsLoadedMsg{findings}
	}
}

func (a App) startECRScan() (App, tea.Cmd) {
	img := a.ecrImagesView.SelectedImage()
	if img == nil {
		return a, nil
	}
	repoName := a.ecrImagesView.RepoName()
	client := a.client
	digest := img.Digest
	tags := img.Tags
	a.loading = true
	return a, func() tea.Msg {
		err := client.StartECRScan(context.Background(), repoName, digest, tags)
		if err != nil {
			return errMsg{err}
		}
		tagLabel := digest[:19]
		if len(tags) > 0 {
			tagLabel = tags[0]
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
	client := a.client
	repoName := a.ecrImagesView.RepoName()
	digest := img.Digest
	return func() tea.Msg {
		err := client.DeleteECRImage(context.Background(), repoName, digest)
		if err != nil {
			return errMsg{err}
		}
		return ecrActionDoneMsg{fmt.Sprintf("Deleted image %s", digest[:19])}
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
	uri := aws.ECRImageURI(repoURI, tag)
	if err := clipboard.WriteAll(uri); err != nil {
		a.err = fmt.Errorf("clipboard: %w", err)
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Copied: %s", uri)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) refreshECRRepos() tea.Cmd {
	client := a.client
	return func() tea.Msg {
		repos, err := client.ListECRRepos(context.Background(), "")
		if err != nil {
			return errMsg{err}
		}
		return ecrReposLoadedMsg{repos}
	}
}

func (a App) refreshECRImages() tea.Cmd {
	repoName := a.ecrImagesView.RepoName()
	client := a.client
	return func() tea.Msg {
		images, err := client.ListECRImages(context.Background(), repoName)
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
