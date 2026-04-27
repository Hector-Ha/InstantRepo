//go:build windows

package desktop

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"instantrepo/internal/domain"
	"instantrepo/internal/service"
)

func Run() {
	ui := &desktopUI{
		service: service.NewAppService(),
	}
	if err := ui.build(); err != nil {
		panic(err)
	}
	ui.mw.Run()
}

type desktopUI struct {
	service *service.AppService

	mw             *walk.MainWindow
	repoEdit       *walk.LineEdit
	folderEdit     *walk.LineEdit
	statusLabel    *walk.Label
	pathLabel      *walk.Label
	summaryEdit    *walk.TextEdit
	requirementsEd *walk.TextEdit
	envGuideEdit   *walk.TextEdit
	envContentEdit *walk.TextEdit
	logsEdit       *walk.TextEdit
	stepList       *walk.ListBox
	stepDetailsEd  *walk.TextEdit
	importButton   *walk.PushButton
	analyzeButton  *walk.PushButton
	refreshButton  *walk.PushButton
	generateEnvBtn *walk.PushButton
	saveEnvBtn     *walk.PushButton
	runStepBtn     *walk.PushButton

	current      *domain.AnalyzeResponse
	currentSteps []domain.ExecutionStep
}

func (ui *desktopUI) build() error {
	return MainWindow{
		AssignTo: &ui.mw,
		Title:    "InstantRepo Desktop",
		MinSize:  Size{Width: 1280, Height: 900},
		Layout:   VBox{MarginsZero: false},
		Children: []Widget{
			GroupBox{
				Title:  "Import Workflow",
				Layout: Grid{Columns: 4},
				Children: []Widget{
					Label{Text: "Repository URL"},
					LineEdit{AssignTo: &ui.repoEdit, ColumnSpan: 3},

					Label{Text: "Destination Folder"},
					LineEdit{AssignTo: &ui.folderEdit, ColumnSpan: 2},
					PushButton{
						Text:      "Choose Folder",
						OnClicked: ui.chooseFolder,
					},
					HSpacer{},

					Composite{
						ColumnSpan: 4,
						Layout:     HBox{},
						Children: []Widget{
							PushButton{
								AssignTo:  &ui.importButton,
								Text:      "Clone && Analyze",
								OnClicked: ui.cloneAndAnalyze,
							},
							PushButton{
								AssignTo:  &ui.analyzeButton,
								Text:      "Analyze Existing Folder",
								OnClicked: ui.analyzeExistingFolder,
							},
							PushButton{
								AssignTo:  &ui.refreshButton,
								Text:      "Refresh",
								OnClicked: ui.refreshAnalysis,
							},
							HSpacer{},
							PushButton{
								AssignTo:  &ui.generateEnvBtn,
								Text:      "Generate / Refresh .env Draft",
								OnClicked: ui.generateEnvDraft,
							},
							PushButton{
								AssignTo:  &ui.saveEnvBtn,
								Text:      "Save Edited .env",
								OnClicked: ui.saveRawEnv,
							},
						},
					},
				},
			},
			Label{AssignTo: &ui.statusLabel, Text: "Waiting for a repository URL and destination folder."},
			Label{AssignTo: &ui.pathLabel, Text: "No repository imported yet."},
			TabWidget{
				Pages: []TabPage{
					{
						Title:  "Overview",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "Repository Summary"},
							TextEdit{AssignTo: &ui.summaryEdit, ReadOnly: true, VScroll: true},
							Label{Text: "Requirements, Services, and Unknowns"},
							TextEdit{AssignTo: &ui.requirementsEd, ReadOnly: true, VScroll: true},
						},
					},
					{
						Title:  "Env Setup",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "Env Guidance"},
							TextEdit{AssignTo: &ui.envGuideEdit, ReadOnly: true, VScroll: true},
							Label{Text: "Env File Editor"},
							TextEdit{AssignTo: &ui.envContentEdit, VScroll: true},
						},
					},
					{
						Title:  "Steps",
						Layout: VBox{},
						Children: []Widget{
							Label{Text: "Execution Plan"},
							ListBox{
								AssignTo:              &ui.stepList,
								OnCurrentIndexChanged: ui.renderSelectedStep,
							},
							TextEdit{AssignTo: &ui.stepDetailsEd, ReadOnly: true, VScroll: true},
							Composite{
								Layout: HBox{},
								Children: []Widget{
									HSpacer{},
									PushButton{
										AssignTo:  &ui.runStepBtn,
										Text:      "Run Selected Step",
										OnClicked: ui.runSelectedStep,
									},
								},
							},
						},
					},
					{
						Title:  "Logs",
						Layout: VBox{},
						Children: []Widget{
							TextEdit{AssignTo: &ui.logsEdit, ReadOnly: true, VScroll: true},
						},
					},
				},
			},
		},
	}.Create()
}

func (ui *desktopUI) chooseFolder() {
	dlg := new(walk.FileDialog)
	dlg.Title = "Choose Destination Folder"
	if ok, err := dlg.ShowBrowseFolder(ui.mw); err != nil {
		ui.showError("Choose Folder Failed", err)
	} else if ok {
		ui.folderEdit.SetText(dlg.FilePath)
	}
}

func (ui *desktopUI) cloneAndAnalyze() {
	repoURL := strings.TrimSpace(ui.repoEdit.Text())
	folder := strings.TrimSpace(ui.folderEdit.Text())
	if repoURL == "" || folder == "" {
		walk.MsgBox(ui.mw, "Missing Input", "Enter a repository URL and choose a destination folder first.", walk.MsgBoxIconInformation)
		return
	}

	ui.setBusy(true, "Cloning repository and analyzing setup requirements...")
	ui.appendLog(fmt.Sprintf("[%s] Clone and analyze started for %s", timestamp(), repoURL))

	go func() {
		resp, err := ui.service.ImportRepository(context.Background(), repoURL, folder)
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Clone and Analyze Failed", err)
				return
			}
			ui.current = &resp
			ui.renderAnalysis()
			ui.appendLog(fmt.Sprintf("[%s] Repository cloned to %s", timestamp(), resp.Source.Path))
			ui.setStatus(fmt.Sprintf("Repository cloned and analyzed. Review env setup, services, and steps for %s.", resp.Analysis.ProjectName))
		})
	}()
}

func (ui *desktopUI) analyzeExistingFolder() {
	folder := strings.TrimSpace(ui.folderEdit.Text())
	if folder == "" {
		walk.MsgBox(ui.mw, "Missing Folder", "Choose or enter a local repository folder first.", walk.MsgBoxIconInformation)
		return
	}

	ui.setBusy(true, "Analyzing existing local repository...")
	go func() {
		resp, err := ui.service.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: folder})
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Analyze Failed", err)
				return
			}
			ui.current = &resp
			ui.renderAnalysis()
			ui.appendLog(fmt.Sprintf("[%s] Analysis completed for %s", timestamp(), resp.Source.Path))
			ui.setStatus("Local repository analyzed successfully.")
		})
	}()
}

func (ui *desktopUI) refreshAnalysis() {
	if ui.current == nil {
		return
	}

	ui.setBusy(true, "Refreshing repository analysis...")
	go func() {
		resp, err := ui.service.Analyze(context.Background(), domain.AnalyzeRequest{LocalPath: ui.current.Source.Path})
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Refresh Failed", err)
				return
			}
			if ui.current != nil && ui.current.Source.RepoURL != "" {
				resp.Source = ui.current.Source
			}
			ui.current = &resp
			ui.renderAnalysis()
			ui.appendLog(fmt.Sprintf("[%s] Analysis refreshed for %s", timestamp(), resp.Source.Path))
			ui.setStatus("Repository analysis refreshed.")
		})
	}()
}

func (ui *desktopUI) generateEnvDraft() {
	if ui.current == nil {
		walk.MsgBox(ui.mw, "No Repository", "Import or analyze a repository before generating an env draft.", walk.MsgBoxIconInformation)
		return
	}

	ui.setBusy(true, "Generating or refreshing the local .env draft...")
	go func() {
		resp, err := ui.service.SaveEnvValues(context.Background(), ui.current.Source.Path, nil)
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Generate Env Draft Failed", err)
				return
			}
			ui.applyExecuteResponse(resp)
			ui.loadEnvFile()
			ui.appendExecutionResult(resp.Result)
			ui.setStatus(".env draft prepared. Review instructions and paste any external secrets into the editor.")
		})
	}()
}

func (ui *desktopUI) saveRawEnv() {
	if ui.current == nil {
		walk.MsgBox(ui.mw, "No Repository", "Import or analyze a repository before saving an env file.", walk.MsgBoxIconInformation)
		return
	}

	ui.setBusy(true, "Saving the edited .env file...")
	go func() {
		resp, err := ui.service.SaveRawEnv(context.Background(), ui.current.Source.Path, ui.envContentEdit.Text())
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Save Env Failed", err)
				return
			}
			ui.applyExecuteResponse(resp)
			ui.loadEnvFile()
			ui.appendExecutionResult(resp.Result)
			ui.setStatus(".env file saved successfully.")
		})
	}()
}

func (ui *desktopUI) runSelectedStep() {
	if ui.current == nil {
		return
	}
	index := ui.stepList.CurrentIndex()
	if index < 0 || index >= len(ui.currentSteps) {
		walk.MsgBox(ui.mw, "No Step Selected", "Choose a step from the list first.", walk.MsgBoxIconInformation)
		return
	}

	step := ui.currentSteps[index]
	ui.setBusy(true, fmt.Sprintf("Running step: %s", step.Title))
	ui.appendLog(fmt.Sprintf("[%s] Running step %s", timestamp(), step.ID))

	go func() {
		resp, err := ui.service.Execute(context.Background(), domain.ExecuteRequest{
			LocalPath:    ui.current.Source.Path,
			StepID:       step.ID,
			ApproveRisky: true,
		})
		ui.mw.Synchronize(func() {
			ui.setBusy(false, "")
			if err != nil {
				ui.showError("Run Step Failed", err)
				return
			}
			ui.applyExecuteResponse(resp)
			ui.loadEnvFile()
			ui.appendExecutionResult(resp.Result)
			if resp.Result.Succeeded {
				ui.setStatus(fmt.Sprintf("Step %s completed successfully.", step.Title))
			} else {
				ui.setStatus(fmt.Sprintf("Step %s finished with exit code %d.", step.Title, resp.Result.ExitCode))
			}
		})
	}()
}

func (ui *desktopUI) renderAnalysis() {
	if ui.current == nil {
		return
	}

	ui.pathLabel.SetText(fmt.Sprintf("Working repo: %s", ui.current.Source.Path))
	_ = ui.summaryEdit.SetText(summaryText(ui.current))
	_ = ui.requirementsEd.SetText(requirementsText(ui.current))
	_ = ui.envGuideEdit.SetText(envGuideText(ui.current))

	ui.currentSteps = append([]domain.ExecutionStep{}, ui.current.Plan.Steps...)
	stepTitles := make([]string, 0, len(ui.currentSteps))
	for _, step := range ui.currentSteps {
		stepTitles = append(stepTitles, fmt.Sprintf("%s [%s | %s]", step.Title, step.Importance, step.Type))
	}
	ui.stepList.SetModel(stepTitles)
	if len(stepTitles) > 0 {
		_ = ui.stepList.SetCurrentIndex(0)
	}
	ui.renderSelectedStep()
	ui.loadEnvFile()
}

func (ui *desktopUI) renderSelectedStep() {
	index := ui.stepList.CurrentIndex()
	if index < 0 || index >= len(ui.currentSteps) {
		_ = ui.stepDetailsEd.SetText("No step selected.")
		ui.runStepBtn.SetEnabled(false)
		return
	}

	step := ui.currentSteps[index]
	command := step.Command
	if strings.HasPrefix(command, "instantrepo internal:") {
		command = "Handled internally by the desktop app."
	}
	if strings.HasPrefix(strings.ToLower(command), "manual ") {
		command = "Manual review required."
	}

	details := []string{
		fmt.Sprintf("Title: %s", step.Title),
		fmt.Sprintf("ID: %s", step.ID),
		fmt.Sprintf("Type: %s", step.Type),
		fmt.Sprintf("Importance: %s", step.Importance),
		fmt.Sprintf("Risk: %s", step.Risk),
		fmt.Sprintf("Requires approval: %t", step.RequiresApproval),
		fmt.Sprintf("Evidence source: %s", step.EvidenceSource),
		fmt.Sprintf("Confidence: %.2f", step.Confidence),
		fmt.Sprintf("Command: %s", command),
		"",
		"Reason:",
		step.Reason,
	}
	if len(step.ConfirmedBy) > 0 {
		details = append(details, "", "Confirmed by:")
		for _, item := range step.ConfirmedBy {
			details = append(details, "- "+item)
		}
	}
	_ = ui.stepDetailsEd.SetText(strings.Join(details, "\r\n"))

	canRun := !strings.Contains(step.Type, "review") && !strings.HasPrefix(strings.ToLower(step.Command), "manual ")
	ui.runStepBtn.SetEnabled(canRun)
}

func (ui *desktopUI) loadEnvFile() {
	if ui.current == nil || ui.current.Plan.Env.TargetPath == "" {
		_ = ui.envContentEdit.SetText("")
		return
	}

	raw, err := os.ReadFile(ui.current.Plan.Env.TargetPath)
	if err != nil {
		_ = ui.envContentEdit.SetText("")
		return
	}
	_ = ui.envContentEdit.SetText(string(raw))
}

func (ui *desktopUI) applyExecuteResponse(resp domain.ExecuteResponse) {
	source := resp.Source
	if ui.current != nil && ui.current.Source.RepoURL != "" && source.RepoURL == "" {
		source = ui.current.Source
	}
	ui.current = &domain.AnalyzeResponse{
		Source:      source,
		Analysis:    resp.Analysis,
		Environment: resp.Environment,
		Plan:        resp.Plan,
	}
	ui.renderAnalysis()
}

func (ui *desktopUI) setBusy(busy bool, status string) {
	ui.importButton.SetEnabled(!busy)
	ui.analyzeButton.SetEnabled(!busy)
	ui.refreshButton.SetEnabled(!busy)
	ui.generateEnvBtn.SetEnabled(!busy)
	ui.saveEnvBtn.SetEnabled(!busy)
	ui.runStepBtn.SetEnabled(!busy)
	if status != "" {
		ui.setStatus(status)
	}
}

func (ui *desktopUI) setStatus(message string) {
	ui.statusLabel.SetText(message)
}

func (ui *desktopUI) showError(title string, err error) {
	ui.appendLog(fmt.Sprintf("[%s] ERROR: %v", timestamp(), err))
	ui.setStatus(err.Error())
	walk.MsgBox(ui.mw, title, err.Error(), walk.MsgBoxIconError)
}

func (ui *desktopUI) appendExecutionResult(result domain.ExecutionResult) {
	lines := []string{
		fmt.Sprintf("[%s] Step %s finished. success=%t exit=%d", timestamp(), result.StepID, result.Succeeded, result.ExitCode),
	}
	if strings.TrimSpace(result.Stdout) != "" {
		lines = append(lines, "STDOUT:", result.Stdout)
	}
	if strings.TrimSpace(result.Stderr) != "" {
		lines = append(lines, "STDERR:", result.Stderr)
	}
	ui.appendLog(strings.Join(lines, "\r\n"))
}

func (ui *desktopUI) appendLog(message string) {
	current := ui.logsEdit.Text()
	if current != "" {
		current += "\r\n\r\n"
	}
	_ = ui.logsEdit.SetText(current + message)
}

func summaryText(resp *domain.AnalyzeResponse) string {
	lines := []string{
		fmt.Sprintf("Project: %s", resp.Analysis.ProjectName),
		fmt.Sprintf("Source: %s", resp.Source.Type),
		fmt.Sprintf("Project type: %s", resp.Analysis.ProjectType),
		fmt.Sprintf("Confidence: %.2f", resp.Analysis.Confidence),
		fmt.Sprintf("Repo path: %s", resp.Source.Path),
		"",
		"Evidence:",
	}
	if len(resp.Analysis.Evidence) == 0 {
		lines = append(lines, "- No strong evidence collected yet.")
	} else {
		for _, item := range resp.Analysis.Evidence {
			lines = append(lines, "- "+item)
		}
	}
	return strings.Join(lines, "\r\n")
}

func requirementsText(resp *domain.AnalyzeResponse) string {
	lines := []string{"Tool requirements and services:"}
	if len(resp.Plan.Gaps) == 0 {
		lines = append(lines, "- No tool gaps detected.")
	} else {
		for _, gap := range resp.Plan.Gaps {
			line := fmt.Sprintf("- %s: %s", gap.Tool, gap.Status)
			if gap.RequiredVersion != "" {
				line += fmt.Sprintf(" | required %s", gap.RequiredVersion)
			}
			if gap.InstalledVersion != "" {
				line += fmt.Sprintf(" | installed %s", gap.InstalledVersion)
			}
			lines = append(lines, line)
		}
	}

	lines = append(lines, "", "Services:")
	if len(resp.Plan.Services) == 0 {
		lines = append(lines, "- No local or external services detected.")
	} else {
		for _, item := range resp.Plan.Services {
			lines = append(lines, fmt.Sprintf("- %s | scope=%s | provisioning=%s | status=%s", item.Name, item.Scope, item.Provisioning, item.Status))
			if item.Details != "" {
				lines = append(lines, "  "+item.Details)
			}
			for _, instruction := range item.Instructions {
				lines = append(lines, "  - "+instruction)
			}
		}
	}

	lines = append(lines, "", "Unknowns:")
	if len(resp.Plan.Unknowns) == 0 {
		lines = append(lines, "- No major unknowns currently flagged.")
	} else {
		for _, item := range resp.Plan.Unknowns {
			lines = append(lines, "- "+item)
		}
	}
	return strings.Join(lines, "\r\n")
}

func envGuideText(resp *domain.AnalyzeResponse) string {
	if len(resp.Plan.Env.Variables) == 0 {
		return "No env template or env variable requirements detected."
	}

	lines := []string{
		fmt.Sprintf("Env target: %s", resp.Plan.Env.TargetPath),
	}
	if resp.Plan.Env.TemplatePath != "" {
		lines = append(lines, fmt.Sprintf("Template source: %s", resp.Plan.Env.TemplatePath))
	}
	lines = append(lines, "")

	for _, item := range resp.Plan.Env.Variables {
		lines = append(lines, fmt.Sprintf("%s | status=%s | strategy=%s | required=%t | secret=%t", item.Name, item.CurrentStatus, item.FillStrategy, item.Required, item.Secret))
		if item.Service != "" {
			lines = append(lines, "  service: "+item.Service)
		}
		if item.SuggestedValue != "" {
			lines = append(lines, "  suggested value: "+item.SuggestedValue)
		}
		for _, instruction := range item.Instructions {
			lines = append(lines, "  - "+instruction)
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\r\n")
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}
