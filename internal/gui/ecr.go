//go:build gui

package gui

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func (w *mainWindow) openECRModule() {
	if w.currentPage == pageECRRepositories {
		return
	}
	if w.guardEditorNavigation(w.openECRModule) {
		return
	}
	w.resourceHistory = nil
	w.loadECRRepositories()
}

func (w *mainWindow) loadECRRepositories() {
	w.resetWorkspaceForBrowserChange()
	w.clearECRRepositories()
	w.clearECRImages()
	w.clearECRFindings()
	w.currentPage = pageECRRepositories
	w.setBreadcrumb("ECR / Repositories")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter repositories…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageECRRepositories)
	w.setDetail("Loading ECR repositories…", detailIntro)
	w.updateActionSensitivity()

	if w.options.ECR == nil {
		w.setDetail("ECR is unavailable because no ECR service was configured.", detailError)
		w.setStatus("ECR service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading ECR repositories…")
	go func() {
		repositories, err := w.options.ECR.ListRepositories(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageECRRepositories {
				return
			}
			w.allECRRepositories = repositories
			w.applyECRRepositoryFilter()
			w.setDetail(ecrRepositoryListSummary(len(repositories)), detailIntro)
		})
	}()
}

func (w *mainWindow) loadECRImages(repository string) {
	repository = strings.TrimSpace(repository)
	if repository == "" || w.options.ECR == nil {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearECRImages()
	w.clearECRFindings()
	w.currentPage = pageECRImages
	w.selectedECRRepository = repository
	w.setBreadcrumb("ECR / " + repository + " / Images")
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter images…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageECRImages)
	w.setDetail("Loading images in "+repository+"…", detailIntro)
	w.updateActionSensitivity()

	ctx, generation := w.startRequest("Loading ECR images in " + repository + "…")
	go func() {
		images, err := w.options.ECR.ListImages(ctx, repository)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageECRImages || w.selectedECRRepository != repository {
				return
			}
			w.allECRImages = images
			w.applyECRImageFilter()
			w.setDetail(ecrImageListSummary(repository, len(images)), detailECRRepository)
		})
	}()
}

func (w *mainWindow) loadECRFindings(image model.ECRImage) {
	if w.selectedECRRepository == "" || image.Digest == "" || w.options.ECR == nil {
		return
	}
	repository, digest := w.selectedECRRepository, image.Digest
	w.resetWorkspaceForBrowserChange()
	w.clearECRFindings()
	w.currentPage = pageECRFindings
	w.selectedECRImage = digest
	w.setBreadcrumb("ECR / " + repository + " / " + ecrImageLabel(image) + " / Findings")
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter findings…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageECRFindings)
	w.setDetail("Loading scan findings for "+ecrImageLabel(image)+"…", detailECRImage)
	w.updateActionSensitivity()

	ctx, generation := w.startRequest("Loading ECR scan findings…")
	go func() {
		findings, err := w.options.ECR.Findings(ctx, repository, digest)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageECRFindings || w.selectedECRRepository != repository || w.selectedECRImage != digest {
				return
			}
			w.allECRFindings = findings
			image = w.mergeECRFindingSummary(image, findings)
			w.applyECRFindingFilter()
			w.setDetail(formatECRImageWithFindingCount(repository, image, len(findings)), detailECRImage)
		})
	}()
}

func (w *mainWindow) mergeECRFindingSummary(image model.ECRImage, findings []model.ECRFinding) model.ECRImage {
	counts := make(map[string]int32)
	for _, finding := range findings {
		counts[strings.ToUpper(finding.Severity)]++
	}
	image.ScanSeverity = counts
	for index := range w.allECRImages {
		if w.allECRImages[index].Digest == image.Digest {
			w.allECRImages[index].ScanSeverity = counts
			break
		}
	}
	return image
}

func (w *mainWindow) clearECRRepositories() {
	w.allECRRepositories = nil
	w.filteredECRRepositories = nil
	w.selectedECRRepository = ""
	if w.ecrRepositoryTable != nil {
		w.ecrRepositoryTable.clear()
	}
}

func (w *mainWindow) clearECRImages() {
	w.allECRImages = nil
	w.filteredECRImages = nil
	w.selectedECRImage = ""
	if w.ecrImageTable != nil {
		w.ecrImageTable.clear()
	}
}

func (w *mainWindow) clearECRFindings() {
	w.allECRFindings = nil
	w.filteredECRFindings = nil
	w.selectedECRFinding = ""
	if w.ecrFindingTable != nil {
		w.ecrFindingTable.clear()
	}
}

func (w *mainWindow) applyECRRepositoryFilter() {
	w.filteredECRRepositories = filterECRRepositories(w.allECRRepositories, w.search.Text())
	rows := make([]string, len(w.filteredECRRepositories))
	for i, repository := range w.filteredECRRepositories {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s", repository.Name, yesNo(repository.ScanOnPush),
			valueOrDash(repository.TagMutability), valueOrDash(repository.EncryptionType), formatTime(repository.CreatedAt))
	}
	w.ecrRepositoryTable.replace(rows)
}

func (w *mainWindow) applyECRImageFilter() {
	w.filteredECRImages = filterECRImages(w.allECRImages, w.search.Text())
	rows := make([]string, len(w.filteredECRImages))
	for i, image := range w.filteredECRImages {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d / %d", ecrImageTags(image), shortECRDigest(image.Digest),
			formatTime(image.PushedAt), formatByteSize(image.SizeBytes), valueOrDash(image.ScanStatus),
			image.ScanSeverity["CRITICAL"], image.ScanSeverity["HIGH"])
	}
	w.ecrImageTable.replace(rows)
}

func (w *mainWindow) applyECRFindingFilter() {
	w.filteredECRFindings = filterECRFindings(w.allECRFindings, w.search.Text())
	rows := make([]string, len(w.filteredECRFindings))
	for i, finding := range w.filteredECRFindings {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s", valueOrDash(finding.Severity), valueOrDash(finding.Name),
			valueOrDash(finding.Package), valueOrDash(finding.Version))
	}
	w.ecrFindingTable.replace(rows)
}

func (w *mainWindow) selectECRRepositoryRow() {
	if w.currentPage != pageECRRepositories {
		return
	}
	position := w.ecrRepositoryTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredECRRepositories) {
		w.selectedECRRepository = ""
		w.setBreadcrumb("ECR / Repositories")
		w.setDetail(ecrRepositoryListSummary(len(w.allECRRepositories)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	repository := w.filteredECRRepositories[position]
	if w.selectedECRRepository != repository.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedECRRepository = repository.Name
	w.setBreadcrumb("ECR / Repositories / " + repository.Name)
	w.setDetail(formatECRRepository(repository), detailECRRepository)
	w.updateActionSensitivity()
}

func (w *mainWindow) openECRRepositoryAt(position uint) {
	if int(position) >= len(w.filteredECRRepositories) {
		return
	}
	repository := w.filteredECRRepositories[position]
	w.ecrRepositoryTable.selection.SetSelected(position)
	w.loadECRImages(repository.Name)
}

func (w *mainWindow) selectECRImageRow() {
	if w.currentPage != pageECRImages {
		return
	}
	position := w.ecrImageTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredECRImages) {
		w.selectedECRImage = ""
		w.setBreadcrumb("ECR / " + w.selectedECRRepository + " / Images")
		w.setDetail(ecrImageListSummary(w.selectedECRRepository, len(w.allECRImages)), detailECRRepository)
		w.updateActionSensitivity()
		return
	}
	image := w.filteredECRImages[position]
	if w.selectedECRImage != image.Digest {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedECRImage = image.Digest
	w.setBreadcrumb("ECR / " + w.selectedECRRepository + " / " + ecrImageLabel(image))
	w.setDetail(formatECRImage(w.selectedECRRepository, image), detailECRImage)
	w.updateActionSensitivity()
}

func (w *mainWindow) openECRImageAt(position uint) {
	if int(position) >= len(w.filteredECRImages) {
		return
	}
	image := w.filteredECRImages[position]
	w.ecrImageTable.selection.SetSelected(position)
	w.loadECRFindings(image)
}

func (w *mainWindow) selectECRFindingRow() {
	if w.currentPage != pageECRFindings {
		return
	}
	position := w.ecrFindingTable.selection.Selected()
	image, _ := findECRImage(w.allECRImages, w.selectedECRImage)
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredECRFindings) {
		w.selectedECRFinding = ""
		w.setDetail(formatECRImageWithFindingCount(w.selectedECRRepository, image, len(w.allECRFindings)), detailECRImage)
		return
	}
	finding := w.filteredECRFindings[position]
	w.selectedECRFinding = ecrFindingKey(finding)
	w.setDetail(formatECRFinding(w.selectedECRRepository, image, finding), detailECRFinding)
}

func (w *mainWindow) openECRFindingAt(position uint) {
	w.ecrFindingTable.selection.SetSelected(position)
}

func (w *mainWindow) refreshECR(foreground bool) {
	if w.options.ECR == nil {
		return
	}
	switch w.currentPage {
	case pageECRRepositories:
		selected := w.selectedECRRepository
		ctx, generation := w.startRefreshRequest("Refreshing ECR repositories…", foreground)
		go func() {
			repositories, err := w.options.ECR.ListRepositories(ctx, "")
			w.finishRefreshRequest(ctx, generation, err, foreground, func() {
				w.allECRRepositories = repositories
				w.applyECRRepositoryFilter()
				if repository, found := findECRRepository(repositories, selected); found {
					w.setDetail(formatECRRepository(repository), detailECRRepository)
					return
				}
				w.selectedECRRepository = ""
				w.setBreadcrumb("ECR / Repositories")
				w.setDetail(ecrRepositoryListSummary(len(repositories)), detailIntro)
			})
		}()
	case pageECRImages:
		w.refreshECRImages(foreground)
	case pageECRFindings:
		w.refreshECRFindings(foreground)
	}
}

func (w *mainWindow) refreshECRImages(foreground bool) {
	repository, selected := w.selectedECRRepository, w.selectedECRImage
	ctx, generation := w.startRefreshRequest("Refreshing ECR images in "+repository+"…", foreground)
	go func() {
		images, err := w.options.ECR.ListImages(ctx, repository)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allECRImages = images
			w.applyECRImageFilter()
			if image, found := findECRImage(images, selected); found {
				w.setDetail(formatECRImage(repository, image), detailECRImage)
				return
			}
			w.selectedECRImage = ""
			w.setBreadcrumb("ECR / " + repository + " / Images")
			w.setDetail(ecrImageListSummary(repository, len(images)), detailECRRepository)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) refreshECRFindings(foreground bool) {
	repository, digest, selected := w.selectedECRRepository, w.selectedECRImage, w.selectedECRFinding
	ctx, generation := w.startRefreshRequest("Refreshing ECR scan findings…", foreground)
	go func() {
		findings, err := w.options.ECR.Findings(ctx, repository, digest)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allECRFindings = findings
			w.applyECRFindingFilter()
			image, _ := findECRImage(w.allECRImages, digest)
			if finding, found := findECRFinding(findings, selected); found {
				w.setDetail(formatECRFinding(repository, image, finding), detailECRFinding)
				return
			}
			w.selectedECRFinding = ""
			w.setDetail(formatECRImageWithFindingCount(repository, image, len(findings)), detailECRImage)
		})
	}()
}

func (w *mainWindow) selectedECRImageValue() (model.ECRImage, bool) {
	if w.selectedECRImage == "" || (w.currentPage != pageECRImages && w.currentPage != pageECRFindings) {
		return model.ECRImage{}, false
	}
	return findECRImage(w.allECRImages, w.selectedECRImage)
}

func (w *mainWindow) copySelectedECRImageURI() {
	image, found := w.selectedECRImageValue()
	repository, repositoryFound := findECRRepository(w.allECRRepositories, w.selectedECRRepository)
	if !found || !repositoryFound || w.options.ECR == nil {
		return
	}
	uri, err := w.options.ECR.ImageURI(repository.URI, image)
	if err != nil {
		w.setStatus(err.Error(), true)
		return
	}
	w.ecrImageTable.view.Clipboard().SetText(uri)
	w.setStatus("Copied image URI "+uri, false)
}

func (w *mainWindow) startSelectedECRScan() {
	image, found := w.selectedECRImageValue()
	if !found || !canStartECRScan(image) || w.selectedECRRepository == "" || w.ecrActionPending || w.options.ECR == nil {
		return
	}
	repository, digest := w.selectedECRRepository, image.Digest
	w.ecrActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Starting image scan for " + ecrImageLabel(image) + "…")
	go func() {
		err := w.options.ECR.StartScan(ctx, repository, image)
		if err != nil {
			w.finishECRAction(ctx, generation, err, "", nil)
			return
		}
		images, refreshErr := w.options.ECR.ListImages(ctx, repository)
		success := "Started image scan for " + ecrImageLabel(image)
		if refreshErr != nil {
			success += " • status refresh failed: " + refreshErr.Error()
		}
		w.finishECRAction(ctx, generation, nil, success, func() {
			if refreshErr == nil {
				w.allECRImages = images
				w.applyECRImageFilter()
				if refreshed, stillPresent := findECRImage(images, digest); stillPresent && w.currentPage == pageECRImages {
					w.selectedECRImage = digest
					w.setDetail(formatECRImage(repository, refreshed), detailECRImage)
				}
			}
		})
	}()
}

func (w *mainWindow) confirmDeleteECRImage() {
	image, found := w.selectedECRImageValue()
	if !found || w.selectedECRRepository == "" || w.ecrActionPending || w.options.ECR == nil {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Delete ECR image")
	dialog.SetMarkup("Delete <b>" + html.EscapeString(ecrImageLabel(image)) + "</b> from <b>" + html.EscapeString(w.selectedECRRepository) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "The image digest "+image.Digest+" will be removed. This cannot be undone.")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseCancel))
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.deleteECRImage(image)
		}
	})
	dialog.Present()
}

func (w *mainWindow) deleteECRImage(image model.ECRImage) {
	if w.ecrActionPending || w.options.ECR == nil {
		return
	}
	repository, digest := w.selectedECRRepository, image.Digest
	existingImages := append([]model.ECRImage(nil), w.allECRImages...)
	w.ecrActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Deleting ECR image " + ecrImageLabel(image) + "…")
	go func() {
		err := w.options.ECR.DeleteImage(ctx, repository, digest)
		if err != nil {
			w.finishECRAction(ctx, generation, err, "", nil)
			return
		}
		images, refreshErr := w.options.ECR.ListImages(ctx, repository)
		success := "Deleted ECR image " + ecrImageLabel(image)
		if refreshErr != nil {
			images = withoutECRImage(existingImages, digest)
			success += " • repository refresh failed: " + refreshErr.Error()
		}
		w.finishECRAction(ctx, generation, nil, success, func() {
			w.currentPage = pageECRImages
			w.clearECRFindings()
			w.selectedECRImage = ""
			w.allECRImages = images
			w.search.SetText("")
			w.search.SetPlaceholderText("Filter images…")
			w.resourceStack.SetVisibleChildName(pageECRImages)
			w.backButton.SetSensitive(true)
			w.applyECRImageFilter()
			w.setBreadcrumb("ECR / " + repository + " / Images")
			w.setDetail(ecrImageListSummary(repository, len(images)), detailECRRepository)
		})
	}()
}

func (w *mainWindow) finishECRAction(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.ecrActionPending = false
		if err != nil {
			w.updateActionSensitivity()
			w.setStatus(err.Error(), true)
			return
		}
		if apply != nil {
			apply()
		}
		w.lastSuccessfulLoad = time.Now()
		w.updateActionSensitivity()
		w.setStatus(success, false)
	})
}

func canStartECRScan(image model.ECRImage) bool {
	if strings.TrimSpace(image.Digest) == "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(image.ScanStatus)) {
	case "ACTIVE", "IN_PROGRESS", "PENDING":
		return false
	default:
		return true
	}
}

func filterECRRepositories(repositories []model.ECRRepo, query string) []model.ECRRepo {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return repositories
	}
	filtered := make([]model.ECRRepo, 0, len(repositories))
	for _, repository := range repositories {
		if strings.Contains(strings.ToLower(repository.Name+" "+repository.URI+" "+repository.ARN), query) {
			filtered = append(filtered, repository)
		}
	}
	return filtered
}

func filterECRImages(images []model.ECRImage, query string) []model.ECRImage {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return images
	}
	filtered := make([]model.ECRImage, 0, len(images))
	for _, image := range images {
		if strings.Contains(strings.ToLower(image.Digest+" "+strings.Join(image.Tags, " ")+" "+image.ScanStatus), query) {
			filtered = append(filtered, image)
		}
	}
	return filtered
}

func filterECRFindings(findings []model.ECRFinding, query string) []model.ECRFinding {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return findings
	}
	filtered := make([]model.ECRFinding, 0, len(findings))
	for _, finding := range findings {
		haystack := strings.Join([]string{finding.Severity, finding.Name, finding.Package, finding.Version, finding.Description, finding.URI}, " ")
		if strings.Contains(strings.ToLower(haystack), query) {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

func findECRRepository(repositories []model.ECRRepo, name string) (model.ECRRepo, bool) {
	for _, repository := range repositories {
		if repository.Name == name {
			return repository, true
		}
	}
	return model.ECRRepo{}, false
}

func findECRImage(images []model.ECRImage, digest string) (model.ECRImage, bool) {
	for _, image := range images {
		if image.Digest == digest {
			return image, true
		}
	}
	return model.ECRImage{}, false
}

func withoutECRImage(images []model.ECRImage, digest string) []model.ECRImage {
	filtered := make([]model.ECRImage, 0, len(images))
	for _, image := range images {
		if image.Digest != digest {
			filtered = append(filtered, image)
		}
	}
	return filtered
}

func findECRFinding(findings []model.ECRFinding, key string) (model.ECRFinding, bool) {
	for _, finding := range findings {
		if ecrFindingKey(finding) == key {
			return finding, true
		}
	}
	return model.ECRFinding{}, false
}

func ecrFindingKey(finding model.ECRFinding) string {
	return strings.Join([]string{finding.Severity, finding.Name, finding.Package, finding.Version}, "\x00")
}

func ecrRepositoryListSummary(count int) string {
	if count == 0 {
		return "No ECR repositories found in this region."
	}
	return fmt.Sprintf("Loaded %d ECR repositories. Select one for details or open it to browse images.", count)
}

func ecrImageListSummary(repository string, count int) string {
	if count == 0 {
		return fmt.Sprintf("ECR REPOSITORY\n\nName    %s\n\nNo images found in this repository.", repository)
	}
	return fmt.Sprintf("ECR REPOSITORY\n\nName    %s\nImages  %d\n\nSelect an image for details or open it to browse scan findings.", repository, count)
}

func formatECRRepository(repository model.ECRRepo) string {
	return fmt.Sprintf("ECR REPOSITORY\n\nName            %s\nURI             %s\nARN             %s\nCreated         %s\nScan on push    %s\nTag mutability  %s\nEncryption      %s",
		repository.Name, valueOrDash(repository.URI), valueOrDash(repository.ARN), formatTime(repository.CreatedAt),
		yesNo(repository.ScanOnPush), valueOrDash(repository.TagMutability), valueOrDash(repository.EncryptionType))
}

func formatECRImage(repository string, image model.ECRImage) string {
	return formatECRImageWithFindingCount(repository, image, -1)
}

func formatECRImageWithFindingCount(repository string, image model.ECRImage, findingCount int) string {
	var out strings.Builder
	fmt.Fprintf(&out, "ECR IMAGE\n\nRepository    %s\nTags          %s\nDigest        %s\nPushed        %s\nSize          %s\nMedia type    %s\nScan status   %s",
		repository, ecrImageTags(image), valueOrDash(image.Digest), formatTime(image.PushedAt), formatByteSize(image.SizeBytes),
		valueOrDash(image.MediaType), valueOrDash(image.ScanStatus))
	if findingCount >= 0 {
		fmt.Fprintf(&out, "\nFindings      %d", findingCount)
	}
	out.WriteString("\n\nSCAN SEVERITY COUNTS")
	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL", "UNDEFINED"} {
		fmt.Fprintf(&out, "\n  %-15s %d", severity, image.ScanSeverity[severity])
	}
	return out.String()
}

func formatECRFinding(repository string, image model.ECRImage, finding model.ECRFinding) string {
	return fmt.Sprintf("ECR SCAN FINDING\n\nRepository   %s\nImage        %s\nSeverity     %s\nFinding      %s\nPackage      %s\nVersion      %s\nReference    %s\n\nDESCRIPTION\n\n%s",
		repository, ecrImageLabel(image), valueOrDash(finding.Severity), valueOrDash(finding.Name),
		valueOrDash(finding.Package), valueOrDash(finding.Version), valueOrDash(finding.URI), valueOrDash(finding.Description))
}

func ecrImageTags(image model.ECRImage) string {
	if len(image.Tags) == 0 {
		return "(untagged)"
	}
	tags := append([]string(nil), image.Tags...)
	sort.Strings(tags)
	return strings.Join(tags, ", ")
}

func ecrImageLabel(image model.ECRImage) string {
	if len(image.Tags) > 0 && image.Tags[0] != "" {
		return image.Tags[0]
	}
	return shortECRDigest(image.Digest)
}

func shortECRDigest(digest string) string {
	const visible = 19
	if len(digest) <= visible {
		return valueOrDash(digest)
	}
	return digest[:visible] + "…"
}
