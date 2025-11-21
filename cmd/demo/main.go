// cmd/demo/main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Corphon/SceneIntruderMCP/internal/app"
	"github.com/Corphon/SceneIntruderMCP/internal/config"
	"github.com/Corphon/SceneIntruderMCP/internal/di"
	"github.com/Corphon/SceneIntruderMCP/internal/models"
	"github.com/Corphon/SceneIntruderMCP/internal/services"
	"github.com/Corphon/SceneIntruderMCP/internal/utils"
)

const defaultConsoleUserID = "console_user"

func main() {
	fmt.Println("🚀 SceneIntruderMCP Console App")
	fmt.Println("=================================")

	// 选择语言
	selectLanguage()

	// 初始化配置
	baseConfig, err := config.Load()
	if err != nil {
		log.Printf("❌ 加载基础配置失败: %v", err)
		return
	}

	// 初始化日志系统
	logFile := fmt.Sprintf("logs/console_%s.log", time.Now().Format("2006-01-02"))
	if err := utils.InitLogger(logFile); err != nil {
		log.Printf("⚠️ 无法初始化结构化日志: %v", err)
		log.Println("继续运行...")
	} else {
		logger := utils.GetLogger()
		logger.Info("Console app starting", nil)
	}

	// 初始化环境
	initializeEnvironment(baseConfig)

	for {
		showMenu()
		choice := getUserInput(T("input_prompt"))

		switch choice {
		case "1", "llm", "ai":
			configureLLM()
		case "2", "scenes":
			manageScenes()
		case "3", "characters":
			manageCharacters()
		case "4", "stories":
			manageStories()
		case "5", "items":
			manageItems()
		case "6", "skills":
			manageSkills()
		case "7", "interact":
			interactWithScene()
		case "8", "export":
			exportStory()
		case "9", "config":
			viewConfig()
		case "10", "status", "stat":
			displayServiceStatus()
		case "11", "services":
			listServices()
		case "0", "quit", "exit":
			fmt.Println(T("goodbye"))
			return
		default:
			fmt.Println(T("invalid_choice"))
		}
		fmt.Println()
	}
}

// 显示菜单
func showMenu() {
	printBox("", fmt.Sprintf("%s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s\n  %s",
		T("menu_title"),
		T("menu_llm"),
		T("menu_scenes"),
		T("menu_characters"),
		T("menu_stories"),
		T("menu_items"),
		T("menu_skills"),
		T("menu_interact"),
		T("menu_export"),
		T("menu_config"),
		T("menu_status"),
		T("menu_services"),
		T("menu_exit")))
}

// 获取用户输入
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return scanner.Text()
}

// 获取用户输入 (带默认值)
func getUserInputWithDefault(prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [默认: %s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultValue
	}
	return input
}

// 1. 初始化项目环境
func initializeEnvironment(cfg *config.Config) {
	fmt.Println("🔧 正在初始化项目环境...")

	// 创建必要的目录
	dirs := []string{
		cfg.DataDir,
		cfg.LogDir,
		cfg.StaticDir,
		cfg.TemplatesDir,
		"temp",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("❌ 创建目录失败 %s: %v", dir, err)
			fmt.Printf("❌ 创建目录失败: %s\n", dir)
			return
		}
	}

	// 初始化配置系统
	if err := config.InitConfig(cfg.DataDir); err != nil {
		log.Printf("❌ 初始化配置系统失败: %v", err)
		fmt.Printf("❌ 初始化配置系统失败: %v\n", err)
		return
	}

	// 初始化服务
	if err := app.InitServices(); err != nil {
		log.Printf("❌ 初始化服务失败: %v", err)
		fmt.Printf("❌ 初始化服务失败: %v\n", err)
		return
	}

	fmt.Println("✅ 项目环境初始化成功！")
	utils.GetLogger().Info("Environment initialized successfully", map[string]interface{}{
		"datadir": cfg.DataDir,
	})
}

// 2. 管理场景
func manageScenes() {
	fmt.Println("🎬 管理场景")
	container := di.GetContainer()
	sceneService := container.Get("scene").(*services.SceneService)
	if sceneService == nil {
		fmt.Println("❌ 场景服务未初始化")
		return
	}

	// 读取现有场景
	scenes, err := sceneService.GetAllScenes()
	if err != nil {
		fmt.Printf("❌ 读取场景失败: %v\n", err)
		return
	}

	fmt.Printf("\n当前共有 %d 个场景:\n", len(scenes))
	if len(scenes) > 0 {
		for i, scene := range scenes {
			fmt.Printf("  %d) %s (%s)\n", i+1, scene.Title, scene.Source)
		}
	} else {
		fmt.Println("  (暂无场景)")
	}

	fmt.Println("\n场景操作:")
	fmt.Println("  c) 创建新场景 (手动输入) — 适合快速录入或调试少量文本")
	fmt.Println("  f) 从文件创建场景 (读取 scenes/create/test.txt) — 复用本地模板，无需AI")
	fmt.Println("  a) 从文件分析创建场景 (使用LLM分析文本并提取角色) ⚡需要AI")
	fmt.Println("  v) 查看场景详情")
	fmt.Println("  d) 删除场景")
	fmt.Println("  b) 返回主菜单")
	fmt.Println()

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "c":
		userID := getUserInputWithDefault("用户ID (可选): ", defaultConsoleUserID)
		title := getUserInput("场景标题: ")
		description := getUserInput("场景描述: ")

		scene, err := sceneService.CreateScene(userID, title, description, "", "")
		if err != nil {
			fmt.Printf("❌ 创建场景失败: %v\n", err)
		} else {
			fmt.Printf("✅ 场景创建成功！ID: %s\n", scene.ID)
		}
	case "f":
		// 从文件创建场景
		content, err := os.ReadFile("scenes/create/test.txt")
		if err != nil {
			fmt.Printf("❌ 读取场景文件失败: %v\n", err)
			fmt.Println("💡 提示: 请确保文件 scenes/create/test.txt 存在")
			return
		}

		sceneContent := string(content)
		fmt.Printf("从文件读取的场景内容:\n%s\n", sceneContent)

		userID := getUserInputWithDefault("用户ID (可选): ", defaultConsoleUserID)
		title := getUserInputWithDefault("场景标题 (默认: 测试场景): ", "测试场景")
		description := getUserInputWithDefault("场景描述 (默认: 从文件创建的场景): ", "从文件创建的场景")

		scene, err := sceneService.CreateScene(userID, title, description, sceneContent, "scenes/create/test.txt")
		if err != nil {
			fmt.Printf("❌ 从文件创建场景失败: %v\n", err)
		} else {
			fmt.Printf("✅ 场景创建成功！ID: %s\n", scene.ID)
		}
	case "a":
		// 从文件分析创建场景 (使用LLM分析文本并提取角色)
		content, err := os.ReadFile("scenes/create/test.txt")
		if err != nil {
			fmt.Printf("❌ 读取场景文件失败: %v\n", err)
			fmt.Println("💡 提示: 请确保文件 scenes/create/test.txt 存在")
			return
		}

		sceneContent := string(content)
		fmt.Printf("从文件读取的场景内容:\n%s\n", sceneContent[:min(200, len(sceneContent))]+"...") // 只显示前200个字符作为预览
		fmt.Println()

		userID := getUserInputWithDefault("用户ID", defaultConsoleUserID)
		title := getUserInputWithDefault("场景标题", "分析场景")

		fmt.Println()
		fmt.Println("正在使用LLM分析文本内容并创建场景...")
		fmt.Println("💡 提示: 此过程需要AI分析，请稍候...")

		// 使用CreateSceneFromText方法，该方法会使用LLM来分析文本并提取角色
		scene, err := sceneService.CreateSceneFromText(userID, sceneContent, title)
		if err != nil {
			fmt.Printf("\n❌ 从文件分析创建场景失败: %v\n", err)
			fmt.Println()
			fmt.Println("💡 可能的原因:")
			fmt.Println("   1. LLM服务未配置 - 请选择菜单选项 7 配置LLM")
			fmt.Println("   2. API密钥无效 - 请检查您的API密钥是否正确")
			fmt.Println("   3. 网络连接问题 - 请检查网络连接")
			fmt.Println("   4. 配额不足 - 请检查您的API配额")
		} else {
			fmt.Printf("\n✅ 从文件分析创建场景成功！ID: %s\n", scene.ID)

			// 尝试读取新创建的场景中的角色
			characters, err := sceneService.GetCharactersByScene(scene.ID)
			if err != nil {
				fmt.Printf("⚠️  读取角色失败: %v\n", err)
			} else {
				fmt.Printf("\n📊 分析出 %d 个角色:\n", len(characters))
				for _, character := range characters {
					fmt.Printf("  - %s (%s)\n", character.Name, character.Role)
				}
			}
		}
	case "v":
		if len(scenes) == 0 {
			fmt.Println("❌ 没有可用的场景")
			return
		}
		sceneNum := getUserInput("输入场景编号查看详情: ")
		if sceneNum == "" {
			return
		}

		// 解析场景编号
		sceneIndex := 0
		if _, err := fmt.Sscanf(sceneNum, "%d", &sceneIndex); err != nil {
			fmt.Println("❌ 无效的场景编号")
			return
		}
		sceneIndex-- // 转换为0基索引

		if sceneIndex < 0 || sceneIndex >= len(scenes) {
			fmt.Println("❌ 场景编号超出范围")
			return
		}

		selectedScene := scenes[sceneIndex]
		fmt.Printf("\n=== 场景详情 ===\n")
		fmt.Printf("ID: %s\n", selectedScene.ID)
		fmt.Printf("标题: %s\n", selectedScene.Title)
		fmt.Printf("描述: %s\n", selectedScene.Description)
		fmt.Printf("来源: %s\n", selectedScene.Source)
		fmt.Printf("创建时间: %s\n", selectedScene.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("内容预览: %s\n", selectedScene.Summary)
	case "d":
		if len(scenes) == 0 {
			fmt.Println("❌ 没有可删除的场景")
			return
		}
		sceneNum := getUserInput("输入要删除的场景编号: ")
		if sceneNum == "" {
			return
		}

		// 解析场景编号
		sceneIndex := 0
		if _, err := fmt.Sscanf(sceneNum, "%d", &sceneIndex); err != nil {
			fmt.Println("❌ 无效的场景编号")
			return
		}
		sceneIndex-- // 转换为0基索引

		if sceneIndex < 0 || sceneIndex >= len(scenes) {
			fmt.Println("❌ 场景编号超出范围")
			return
		}

		sceneToDelete := scenes[sceneIndex]
		confirm := getUserInput(fmt.Sprintf("确认删除场景 '%s' (y/N): ", sceneToDelete.Title))
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			if err := sceneService.DeleteScene(sceneToDelete.ID); err != nil {
				fmt.Printf("❌ 删除场景失败: %v\n", err)
			} else {
				fmt.Printf("✅ 场景 '%s' 删除成功！\n", sceneToDelete.Title)
			}
		} else {
			fmt.Println("❌ 删除操作已取消")
		}
	case "b":
		fmt.Println("🔙 返回主菜单")
		return
	}
}

// 3. 管理角色
func manageCharacters() {
	fmt.Println("👤 管理角色")
	container := di.GetContainer()
	sceneService := container.Get("scene").(*services.SceneService)
	if sceneService == nil {
		fmt.Println("❌ 场景服务未初始化")
		return
	}

	fmt.Println("角色功能菜单:")
	fmt.Println("  l) 列出所有角色")
	fmt.Println("  c) 创建新角色")
	fmt.Println("  u) 更新角色")
	fmt.Println("  v) 查看角色详情")
	fmt.Println("  d) 删除角色")
	fmt.Println("  b) 返回主菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		characters, err := sceneService.GetCharactersByScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取角色失败: %v\n", err)
			return
		}

		if len(characters) == 0 {
			fmt.Println("当前场景中没有角色")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个角色:\n", sceneID, len(characters))
		for i, character := range characters {
			fmt.Printf("  %d) %s (%s) - %s\n", i+1, character.Name, character.Role, character.Description)
		}
	case "c":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		name := getUserInput("角色名称: ")
		description := getUserInput("角色描述: ")
		personality := getUserInput("角色性格: ")
		role := getUserInputWithDefault("角色身份/职业: ", "Unknown")
		background := getUserInput("角色背景: ")
		speechStyle := getUserInput("说话风格: ")

		character := models.Character{
			ID:          fmt.Sprintf("char_%d", time.Now().UnixNano()),
			Name:        name,
			Description: description,
			Personality: personality,
			Role:        role,
			Background:  background,
			SpeechStyle: speechStyle,
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
		}

		if err := sceneService.AddCharacter(sceneID, &character); err != nil {
			fmt.Printf("❌ 添加角色失败: %v\n", err)
		} else {
			fmt.Printf("✅ 角色 '%s' 添加成功！角色ID: %s\n", character.Name, character.ID)
		}
	case "u":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		characters, err := sceneService.GetCharactersByScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取角色失败: %v\n", err)
			return
		}

		if len(characters) == 0 {
			fmt.Println("当前场景中没有角色")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个角色:\n", sceneID, len(characters))
		for i, character := range characters {
			fmt.Printf("  %d) %s (%s)\n", i+1, character.Name, character.Role)
		}

		characterNum := getUserInput("输入要更新的角色编号: ")
		if characterNum == "" {
			return
		}

		// 解析角色编号
		characterIndex := 0
		if _, err := fmt.Sscanf(characterNum, "%d", &characterIndex); err != nil {
			fmt.Println("❌ 无效的角色编号")
			return
		}
		characterIndex-- // 转换为0基索引

		if characterIndex < 0 || characterIndex >= len(characters) {
			fmt.Println("❌ 角色编号超出范围")
			return
		}

		selectedCharacter := characters[characterIndex]
		fmt.Printf("正在更新角色 '%s' (ID: %s)\n", selectedCharacter.Name, selectedCharacter.ID)

		name := getUserInputWithDefault("角色名称 (当前: "+selectedCharacter.Name+"): ", selectedCharacter.Name)
		description := getUserInputWithDefault("角色描述 (当前: "+selectedCharacter.Description+"): ", selectedCharacter.Description)
		personality := getUserInputWithDefault("角色性格 (当前: "+selectedCharacter.Personality+"): ", selectedCharacter.Personality)
		role := getUserInputWithDefault("角色身份/职业 (当前: "+selectedCharacter.Role+"): ", selectedCharacter.Role)
		background := getUserInputWithDefault("角色背景 (当前: "+selectedCharacter.Background+"): ", selectedCharacter.Background)
		speechStyle := getUserInputWithDefault("说话风格 (当前: "+selectedCharacter.SpeechStyle+"): ", selectedCharacter.SpeechStyle)

		updatedCharacter := models.Character{
			ID:          selectedCharacter.ID,
			Name:        name,
			Description: description,
			Personality: personality,
			Role:        role,
			Background:  background,
			SpeechStyle: speechStyle,
			CreatedAt:   selectedCharacter.CreatedAt, // 保留原始创建时间
			LastUpdated: time.Now(),
		}

		if err := sceneService.UpdateCharacter(sceneID, selectedCharacter.ID, &updatedCharacter); err != nil {
			fmt.Printf("❌ 更新角色失败: %v\n", err)
		} else {
			fmt.Printf("✅ 角色 '%s' 更新成功！\n", updatedCharacter.Name)
		}
	case "v":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		characters, err := sceneService.GetCharactersByScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取角色失败: %v\n", err)
			return
		}

		if len(characters) == 0 {
			fmt.Println("当前场景中没有角色")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个角色:\n", sceneID, len(characters))
		for i, character := range characters {
			fmt.Printf("  %d) %s (%s)\n", i+1, character.Name, character.Role)
		}

		characterNum := getUserInput("输入要查看的角色编号: ")
		if characterNum == "" {
			return
		}

		// 解析角色编号
		characterIndex := 0
		if _, err := fmt.Sscanf(characterNum, "%d", &characterIndex); err != nil {
			fmt.Println("❌ 无效的角色编号")
			return
		}
		characterIndex-- // 转换为0基索引

		if characterIndex < 0 || characterIndex >= len(characters) {
			fmt.Println("❌ 角色编号超出范围")
			return
		}

		selectedCharacter := characters[characterIndex]
		fmt.Printf("\n=== 角色详情 ===\n")
		fmt.Printf("ID: %s\n", selectedCharacter.ID)
		fmt.Printf("姓名: %s\n", selectedCharacter.Name)
		fmt.Printf("身份/职业: %s\n", selectedCharacter.Role)
		fmt.Printf("描述: %s\n", selectedCharacter.Description)
		fmt.Printf("性格: %s\n", selectedCharacter.Personality)
		fmt.Printf("背景: %s\n", selectedCharacter.Background)
		fmt.Printf("说话风格: %s\n", selectedCharacter.SpeechStyle)
		fmt.Printf("创建时间: %s\n", selectedCharacter.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("最后更新: %s\n", selectedCharacter.LastUpdated.Format("2006-01-02 15:04:05"))
	case "d":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		characters, err := sceneService.GetCharactersByScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取角色失败: %v\n", err)
			return
		}

		if len(characters) == 0 {
			fmt.Println("当前场景中没有角色")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个角色:\n", sceneID, len(characters))
		for i, character := range characters {
			fmt.Printf("  %d) %s (%s)\n", i+1, character.Name, character.Role)
		}

		characterNum := getUserInput("输入要删除的角色编号: ")
		if characterNum == "" {
			return
		}

		// 解析角色编号
		characterIndex := 0
		if _, err := fmt.Sscanf(characterNum, "%d", &characterIndex); err != nil {
			fmt.Println("❌ 无效的角色编号")
			return
		}
		characterIndex-- // 转换为0基索引

		if characterIndex < 0 || characterIndex >= len(characters) {
			fmt.Println("❌ 角色编号超出范围")
			return
		}

		characterToDelete := characters[characterIndex]
		confirm := getUserInput(fmt.Sprintf("确认删除角色 '%s' (y/N): ", characterToDelete.Name))
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			if err := sceneService.DeleteCharacter(sceneID, characterToDelete.ID); err != nil {
				fmt.Printf("❌ 删除角色失败: %v\n", err)
			} else {
				fmt.Printf("✅ 角色 '%s' 删除成功！\n", characterToDelete.Name)
			}
		} else {
			fmt.Println("❌ 删除操作已取消")
		}
	case "b":
		fmt.Println("🔙 返回主菜单")
		return
	}
}

// 4. 管理故事
func manageStories() {
	fmt.Println("📚 管理故事")
	container := di.GetContainer()
	storyService := container.Get("story").(*services.StoryService)
	sceneService := container.Get("scene").(*services.SceneService)
	if storyService == nil {
		fmt.Println("❌ 故事服务未初始化")
		return
	}

	fmt.Println("故事功能菜单:")
	fmt.Println("  l) 列出所有故事")
	fmt.Println("  c) 创建新故事")
	fmt.Println("  v) 查看故事详情")
	fmt.Println("  u) 更新故事")
	fmt.Println("  d) 删除故事")
	fmt.Println("  n) 管理故事节点")
	fmt.Println("  t) 管理任务")
	fmt.Println("  o) 管理地点")
	fmt.Println("  p) 推进故事")
	fmt.Println("  e) 探索地点")
	fmt.Println("  b) 返回主菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		storyData, err := storyService.GetStoryForScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取故事失败: %v\n", err)
			return
		}

		fmt.Printf("场景 '%s' 的故事详情:\n", sceneID)
		fmt.Printf("  故事介绍: %s\n", storyData.Intro)
		fmt.Printf("  主要目标: %s\n", storyData.MainObjective)
		fmt.Printf("  当前状态: %s\n", storyData.CurrentState)
		fmt.Printf("  进度: %d%%\n", storyData.Progress)
		fmt.Printf("  节点数量: %d\n", len(storyData.Nodes))
		fmt.Printf("  任务数量: %d\n", len(storyData.Tasks))
		fmt.Printf("  地点数量: %d\n", len(storyData.Locations))
		fmt.Printf("  最后更新: %s\n", storyData.LastUpdated.Format("2006-01-02 15:04:05"))

	case "c":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		// 检查场景是否存在
		scene, err := sceneService.LoadScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 指定场景不存在: %v\n", err)
			return
		}

		fmt.Printf("在场景: %s 中创建故事...\n", scene.Scene.Title)

		// 创建用户偏好设置
		preferences := &models.UserPreferences{
			PreferredModel:  "qwen3-max",
			CreativityLevel: models.CreativityExpansive,
		}

		// 初始化故事
		storyData, err := storyService.InitializeStoryForScene(sceneID, preferences)
		if err != nil {
			fmt.Printf("❌ 初始化故事失败: %v\n", err)
			return
		}

		fmt.Printf("✅ 故事初始化成功！\n")
		fmt.Printf("  场景ID: %s\n", storyData.SceneID)
		fmt.Printf("  故事介绍: %s\n", storyData.Intro)
		fmt.Printf("  主要目标: %s\n", storyData.MainObjective)

	case "v":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		storyData, err := storyService.GetStoryForScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取故事失败: %v\n", err)
			return
		}

		fmt.Printf("\n=== 故事详情 ===\n")
		fmt.Printf("场景ID: %s\n", storyData.SceneID)
		fmt.Printf("故事介绍: %s\n", storyData.Intro)
		fmt.Printf("主要目标: %s\n", storyData.MainObjective)
		fmt.Printf("当前状态: %s\n", storyData.CurrentState)
		fmt.Printf("进度: %d%%\n", storyData.Progress)
		fmt.Printf("最后更新: %s\n", storyData.LastUpdated.Format("2006-01-02 15:04:05"))

		// 显示节点信息
		fmt.Printf("\n故事节点 (%d个):\n", len(storyData.Nodes))
		for i, node := range storyData.Nodes {
			status := "隐藏"
			if node.IsRevealed {
				status = "已显示"
			}
			fmt.Printf("  %d) %s (%s) - %s... [来源: %s]\n", i+1, node.ID[:min(12, len(node.ID))], status, node.Content[:min(30, len(node.Content))], node.Source)
		}

		// 显示任务信息
		fmt.Printf("\n任务 (%d个):\n", len(storyData.Tasks))
		for i, task := range storyData.Tasks {
			status := "未完成"
			if task.Completed {
				status = "已完成"
			}
			fmt.Printf("  %d) %s (%s) - %s\n", i+1, task.Title, status, task.Description[:min(30, len(task.Description))])
		}

		// 显示地点信息
		fmt.Printf("\n地点 (%d个):\n", len(storyData.Locations))
		for i, location := range storyData.Locations {
			access := "不可访问"
			if location.Accessible {
				access = "可访问"
			}
			fmt.Printf("  %d) %s (%s) - %s\n", i+1, location.Name, access, location.Description[:min(30, len(location.Description))])
		}

	case "u":
		sceneID := getUserInput(T("enter_scene_id"))
		if sceneID == "" {
			fmt.Println(T("scene_id_empty"))
			return
		}

		storyData, err := storyService.GetStoryForScene(sceneID)
		if err != nil {
			fmt.Println(fmt.Sprintf(T("read_fail"), err))
			return
		}

		fmt.Printf("当前故事状态 - 进度: %d%%, 状态: %s\n", storyData.Progress, storyData.CurrentState)
		fmt.Println("1. 更新进度")
		fmt.Println("2. 更新状态")
		fmt.Println("3. 更新简介")
		fmt.Println("4. 更新目标")

		subChoice := getUserInput("选择更新项: ")
		switch subChoice {
		case "1":
			progStr := getUserInput("新进度 (0-100): ")
			var prog int
			fmt.Sscanf(progStr, "%d", &prog)
			storyData.Progress = prog
		case "2":
			storyData.CurrentState = getUserInput("新状态: ")
		case "3":
			storyData.Intro = getUserInput("新简介: ")
		case "4":
			storyData.MainObjective = getUserInput("新目标: ")
		}

		if err := storyService.SaveStoryData(sceneID, storyData); err != nil {
			fmt.Printf("❌ 更新失败: %v\n", err)
		} else {
			fmt.Println(T("update_success"))
		}

	case "d":
		fmt.Println("故事删除功能待实现...")
		fmt.Println("💡 提示: 故事数据存储在场景目录下，删除场景时会自动删除相关故事数据")

	case "n":
		manageStoryNodes(storyService, sceneService)
	case "t":
		manageStoryTasks(storyService, sceneService)
	case "o":
		manageStoryLocations(storyService, sceneService)
	case "p":
		advanceStory(storyService, sceneService)
	case "e":
		exploreLocations(storyService, sceneService)
	case "b":
		fmt.Println("🔙 返回主菜单")
		return
	}
}

// 管理故事节点
func manageStoryNodes(storyService *services.StoryService, _ *services.SceneService) {
	fmt.Println("📝 管理故事节点")
	sceneID := getUserInput("请输入场景ID: ")
	if sceneID == "" {
		fmt.Println("❌ 场景ID不能为空")
		return
	}

	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		fmt.Printf("❌ 读取故事失败: %v\n", err)
		return
	}

	fmt.Printf("场景 '%s' 共有 %d 个故事节点:\n", sceneID, len(storyData.Nodes))
	for i, node := range storyData.Nodes {
		status := "隐藏"
		if node.IsRevealed {
			status = "已显示"
		}
		fmt.Printf("  %d) %s (%s) - %s... [状态: %s]\n", i+1, node.ID[:min(12, len(node.ID))], node.Type, node.Content[:min(30, len(node.Content))], status)
	}

	fmt.Println("\n节点操作:")
	fmt.Println("  l) 列出节点")
	fmt.Println("  v) 查看节点详情")
	fmt.Println("  b) 返回上级菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		// 重新显示节点列表
		fmt.Printf("场景 '%s' 共有 %d 个故事节点:\n", sceneID, len(storyData.Nodes))
		for i, node := range storyData.Nodes {
			status := "隐藏"
			if node.IsRevealed {
				status = "已显示"
			}
			fmt.Printf("  %d) %s (%s) - %s... [状态: %s]\n", i+1, node.ID[:min(12, len(node.ID))], node.Type, node.Content[:min(30, len(node.Content))], status)
		}
	case "v":
		if len(storyData.Nodes) == 0 {
			fmt.Println("❌ 没有可用的节点")
			return
		}
		nodeNum := getUserInput("输入节点编号查看详情: ")
		if nodeNum == "" {
			return
		}

		// 解析节点编号
		nodeIndex := 0
		if _, err := fmt.Sscanf(nodeNum, "%d", &nodeIndex); err != nil {
			fmt.Println("❌ 无效的节点编号")
			return
		}
		nodeIndex-- // 转换为0基索引

		if nodeIndex < 0 || nodeIndex >= len(storyData.Nodes) {
			fmt.Println("❌ 节点编号超出范围")
			return
		}

		selectedNode := storyData.Nodes[nodeIndex]
		fmt.Printf("\n=== 节点详情 ===\n")
		fmt.Printf("ID: %s\n", selectedNode.ID)
		fmt.Printf("类型: %s\n", selectedNode.Type)
		fmt.Printf("状态: %s\n", map[bool]string{true: "已显示", false: "隐藏"}[selectedNode.IsRevealed])
		fmt.Printf("内容: %s\n", selectedNode.Content)
		fmt.Printf("源: %s\n", selectedNode.Source)
		fmt.Printf("创建时间: %s\n", selectedNode.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("选择数量: %d\n", len(selectedNode.Choices))
		if len(selectedNode.Choices) > 0 {
			fmt.Println("可用选择:")
			for j, choice := range selectedNode.Choices {
				selectionStatus := map[bool]string{true: "已选", false: "未选"}[choice.Selected]
				fmt.Printf("  %d) %s [状态: %s]\n", j+1, choice.Text, selectionStatus)
				if choice.Consequence != "" {
					fmt.Printf("     后果: %s\n", choice.Consequence)
				}
			}
		}
	case "b":
		fmt.Println("🔙 返回上级菜单")
		return
	}
}

// 管理故事任务
func manageStoryTasks(storyService *services.StoryService, _ *services.SceneService) {
	fmt.Println("📋 管理故事任务")
	sceneID := getUserInput("请输入场景ID: ")
	if sceneID == "" {
		fmt.Println("❌ 场景ID不能为空")
		return
	}

	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		fmt.Printf("❌ 读取故事失败: %v\n", err)
		return
	}

	fmt.Printf("场景 '%s' 共有 %d 个任务:\n", sceneID, len(storyData.Tasks))
	for i, task := range storyData.Tasks {
		status := "未完成"
		if task.Completed {
			status = "已完成"
		}
		fmt.Printf("  %d) %s (%s) - %s...\n", i+1, task.Title, status, task.Description[:min(30, len(task.Description))])
	}

	fmt.Println("\n任务操作:")
	fmt.Println("  l) 列出任务")
	fmt.Println("  v) 查看任务详情")
	fmt.Println("  c) 完成任务目标")
	fmt.Println("  b) 返回上级菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		// 重新显示任务列表
		fmt.Printf("场景 '%s' 共有 %d 个任务:\n", sceneID, len(storyData.Tasks))
		for i, task := range storyData.Tasks {
			status := "未完成"
			if task.Completed {
				status = "已完成"
			}
			fmt.Printf("  %d) %s (%s) - %s...\n", i+1, task.Title, status, task.Description[:min(30, len(task.Description))])
		}
	case "v":
		if len(storyData.Tasks) == 0 {
			fmt.Println("❌ 没有可用的任务")
			return
		}
		taskNum := getUserInput("输入任务编号查看详情: ")
		if taskNum == "" {
			return
		}

		// 解析任务编号
		taskIndex := 0
		if _, err := fmt.Sscanf(taskNum, "%d", &taskIndex); err != nil {
			fmt.Println("❌ 无效的任务编号")
			return
		}
		taskIndex-- // 转换为0基索引

		if taskIndex < 0 || taskIndex >= len(storyData.Tasks) {
			fmt.Println("❌ 任务编号超出范围")
			return
		}

		selectedTask := storyData.Tasks[taskIndex]
		fmt.Printf("\n=== 任务详情 ===\n")
		fmt.Printf("ID: %s\n", selectedTask.ID)
		fmt.Printf("标题: %s\n", selectedTask.Title)
		fmt.Printf("描述: %s\n", selectedTask.Description)
		fmt.Printf("奖励: %s\n", selectedTask.Reward)
		fmt.Printf("状态: %s\n", map[bool]string{true: "已显示", false: "隐藏"}[selectedTask.IsRevealed])
		fmt.Printf("完成状态: %s\n", map[bool]string{true: "已完成", false: "未完成"}[selectedTask.Completed])
		fmt.Printf("源: %s\n", selectedTask.Source)
		fmt.Printf("目标数量: %d\n", len(selectedTask.Objectives))
		if len(selectedTask.Objectives) > 0 {
			fmt.Println("任务目标:")
			for j, obj := range selectedTask.Objectives {
				objStatus := map[bool]string{true: "已完成", false: "未完成"}[obj.Completed]
				fmt.Printf("  %d) %s [状态: %s]\n", j+1, obj.Description, objStatus)
			}
		}
	case "c":
		if len(storyData.Tasks) == 0 {
			fmt.Println("❌ 没有可用的任务")
			return
		}
		taskNum := getUserInput("输入任务编号: ")
		if taskNum == "" {
			return
		}

		// 解析任务编号
		taskIndex := 0
		if _, err := fmt.Sscanf(taskNum, "%d", &taskIndex); err != nil {
			fmt.Println("❌ 无效的任务编号")
			return
		}
		taskIndex-- // 转换为0基索引

		if taskIndex < 0 || taskIndex >= len(storyData.Tasks) {
			fmt.Println("❌ 任务编号超出范围")
			return
		}

		selectedTask := storyData.Tasks[taskIndex]

		if len(selectedTask.Objectives) == 0 {
			fmt.Println("❌ 任务没有目标")
			return
		}

		fmt.Printf("任务 '%s' 共有 %d 个目标:\n", selectedTask.Title, len(selectedTask.Objectives))
		for j, obj := range selectedTask.Objectives {
			objStatus := map[bool]string{true: "已完成", false: "未完成"}[obj.Completed]
			fmt.Printf("  %d) %s [状态: %s]\n", j+1, obj.Description, objStatus)
		}

		objNum := getUserInput("输入目标编号完成: ")
		if objNum == "" {
			return
		}

		// 解析目标编号
		objIndex := 0
		if _, err := fmt.Sscanf(objNum, "%d", &objIndex); err != nil {
			fmt.Println("❌ 无效的目标编号")
			return
		}
		objIndex-- // 转换为0基索引

		if objIndex < 0 || objIndex >= len(selectedTask.Objectives) {
			fmt.Println("❌ 目标编号超出范围")
			return
		}

		selectedObjective := selectedTask.Objectives[objIndex]
		if selectedObjective.Completed {
			fmt.Println("❌ 目标已完成")
			return
		}

		confirm := getUserInput(fmt.Sprintf("确认完成目标 '%s' (y/N): ", selectedObjective.Description))
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			if err := storyService.CompleteObjective(sceneID, selectedTask.ID, selectedObjective.ID); err != nil {
				fmt.Printf("❌ 完成目标失败: %v\n", err)
			} else {
				fmt.Printf("✅ 目标 '%s' 完成！\n", selectedObjective.Description)
			}
		} else {
			fmt.Println("❌ 完成操作已取消")
		}
	case "b":
		fmt.Println("🔙 返回上级菜单")
		return
	}
}

// 管理故事地点
func manageStoryLocations(storyService *services.StoryService, _ *services.SceneService) {
	fmt.Println("🗺️ 管理故事地点")
	sceneID := getUserInput("请输入场景ID: ")
	if sceneID == "" {
		fmt.Println("❌ 场景ID不能为空")
		return
	}

	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		fmt.Printf("❌ 读取故事失败: %v\n", err)
		return
	}

	fmt.Printf("场景 '%s' 共有 %d 个地点:\n", sceneID, len(storyData.Locations))
	for i, location := range storyData.Locations {
		access := "不可访问"
		if location.Accessible {
			access = "可访问"
		}
		fmt.Printf("  %d) %s (%s) - %s...\n", i+1, location.Name, access, location.Description[:min(30, len(location.Description))])
	}

	fmt.Println("\n地点操作:")
	fmt.Println("  l) 列出地点")
	fmt.Println("  v) 查看地点详情")
	fmt.Println("  u) 解锁地点")
	fmt.Println("  b) 返回上级菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		// 重新显示地点列表
		fmt.Printf("场景 '%s' 共有 %d 个地点:\n", sceneID, len(storyData.Locations))
		for i, location := range storyData.Locations {
			access := "不可访问"
			if location.Accessible {
				access = "可访问"
			}
			fmt.Printf("  %d) %s (%s) - %s...\n", i+1, location.Name, access, location.Description[:min(30, len(location.Description))])
		}
	case "v":
		if len(storyData.Locations) == 0 {
			fmt.Println("❌ 没有可用的地点")
			return
		}
		locationNum := getUserInput("输入地点编号查看详情: ")
		if locationNum == "" {
			return
		}

		// 解析地点编号
		locationIndex := 0
		if _, err := fmt.Sscanf(locationNum, "%d", &locationIndex); err != nil {
			fmt.Println("❌ 无效的地点编号")
			return
		}
		locationIndex-- // 转换为0基索引

		if locationIndex < 0 || locationIndex >= len(storyData.Locations) {
			fmt.Println("❌ 地点编号超出范围")
			return
		}

		selectedLocation := storyData.Locations[locationIndex]
		fmt.Printf("\n=== 地点详情 ===\n")
		fmt.Printf("ID: %s\n", selectedLocation.ID)
		fmt.Printf("名称: %s\n", selectedLocation.Name)
		fmt.Printf("描述: %s\n", selectedLocation.Description)
		fmt.Printf("访问状态: %s\n", map[bool]string{true: "可访问", false: "不可访问"}[selectedLocation.Accessible])
		fmt.Printf("源: %s\n", selectedLocation.Source)
	case "u":
		if len(storyData.Locations) == 0 {
			fmt.Println("❌ 没有可用的地点")
			return
		}
		locationNum := getUserInput("输入地点编号解锁: ")
		if locationNum == "" {
			return
		}

		// 解析地点编号
		locationIndex := 0
		if _, err := fmt.Sscanf(locationNum, "%d", &locationIndex); err != nil {
			fmt.Println("❌ 无效的地点编号")
			return
		}
		locationIndex-- // 转换为0基索引

		if locationIndex < 0 || locationIndex >= len(storyData.Locations) {
			fmt.Println("❌ 地点编号超出范围")
			return
		}

		selectedLocation := storyData.Locations[locationIndex]
		if selectedLocation.Accessible {
			fmt.Println("❌ 地点已可访问")
			return
		}

		confirm := getUserInput(fmt.Sprintf("确认解锁地点 '%s' (y/N): ", selectedLocation.Name))
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			if err := storyService.UnlockLocation(sceneID, selectedLocation.ID); err != nil {
				fmt.Printf("❌ 解锁地点失败: %v\n", err)
			} else {
				fmt.Printf("✅ 地点 '%s' 解锁成功！\n", selectedLocation.Name)
			}
		} else {
			fmt.Println("❌ 解锁操作已取消")
		}
	case "b":
		fmt.Println("🔙 返回上级菜单")
		return
	}
}

// 推进故事
func advanceStory(storyService *services.StoryService, _ *services.SceneService) {
	fmt.Println("🚀 推进故事")
	sceneID := getUserInput("请输入场景ID: ")
	if sceneID == "" {
		fmt.Println("❌ 场景ID不能为空")
		return
	}

	container := di.GetContainer()
	llmService := container.Get("llm").(*services.LLMService)
	if llmService == nil {
		fmt.Println("❌ LLM服务未初始化，无法推进故事")
		return
	}

	// 创建用户偏好设置
	preferences := &models.UserPreferences{
		PreferredModel:  "qwen3-max",
		CreativityLevel: models.CreativityExpansive,
		AllowPlotTwists: true,
	}

	fmt.Println("正在推进故事...")
	update, err := storyService.AdvanceStory(sceneID, preferences)
	if err != nil {
		fmt.Printf("❌ 推进故事失败: %v\n", err)
		return
	}

	if update != nil {
		fmt.Printf("✅ 故事推进成功！\n")
		fmt.Printf("标题: %s\n", update.Title)
		fmt.Printf("内容: %s\n", update.Content)
		fmt.Printf("类型: %s\n", update.Type)

		// 更新后显示新状态
		storyData, err := storyService.GetStoryForScene(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取故事失败: %v\n", err)
			return
		}

		fmt.Printf("当前进度: %d%%, 状态: %s\n", storyData.Progress, storyData.CurrentState)

		if update.NewTask != nil {
			fmt.Printf("新任务: %s\n", update.NewTask.Title)
		}
		if update.NewClue != "" {
			fmt.Printf("新线索: %s\n", update.NewClue)
		}
	} else {
		fmt.Println("⚠️  未生成新的故事更新")
	}
}

// 探索地点
func exploreLocations(storyService *services.StoryService, _ *services.SceneService) {
	fmt.Println("🔍 探索地点")
	sceneID := getUserInput("请输入场景ID: ")
	if sceneID == "" {
		fmt.Println("❌ 场景ID不能为空")
		return
	}

	storyData, err := storyService.GetStoryForScene(sceneID)
	if err != nil {
		fmt.Printf("❌ 读取故事失败: %v\n", err)
		return
	}

	// 显示可访问的地点
	accessibleLocations := []models.StoryLocation{}
	fmt.Printf("场景 '%s' 的地点:\n", sceneID)
	for i, location := range storyData.Locations {
		access := "不可访问"
		if location.Accessible {
			access = "可访问"
			accessibleLocations = append(accessibleLocations, location)
		}
		fmt.Printf("  %d) %s (%s) - %s...\n", i+1, location.Name, access, location.Description[:min(30, len(location.Description))])
	}

	if len(accessibleLocations) == 0 {
		fmt.Println("❌ 没有可访问的地点")
		return
	}

	// 显示可访问的地点
	fmt.Println("\n可访问的地点:")
	for i, location := range accessibleLocations {
		fmt.Printf("  %d) %s - %s...\n", i+1, location.Name, location.Description[:min(30, len(location.Description))])
	}

	locationNum := getUserInput("输入要探索的地点编号: ")
	if locationNum == "" {
		return
	}

	// 解析地点编号
	locationIndex := 0
	if _, err := fmt.Sscanf(locationNum, "%d", &locationIndex); err != nil {
		fmt.Println("❌ 无效的地点编号")
		return
	}
	locationIndex-- // 转换为0基索引

	if locationIndex < 0 || locationIndex >= len(accessibleLocations) {
		fmt.Println("❌ 地点编号超出范围")
		return
	}

	container := di.GetContainer()
	llmService := container.Get("llm").(*services.LLMService)
	if llmService == nil {
		fmt.Println("❌ LLM服务未初始化，无法探索地点")
		return
	}

	// 创建用户偏好设置
	preferences := &models.UserPreferences{
		PreferredModel:  "qwen3-max",
		CreativityLevel: models.CreativityExpansive,
		AllowPlotTwists: true,
	}

	selectedLocation := accessibleLocations[locationIndex]
	fmt.Printf("正在探索地点 '%s'...\n", selectedLocation.Name)

	result, err := storyService.ExploreLocation(sceneID, selectedLocation.ID, preferences)
	if err != nil {
		fmt.Printf("❌ 探索地点失败: %v\n", err)
		return
	}

	if result != nil {
		fmt.Printf("✅ 探索成功！\n")
		fmt.Printf("探索描述: %s\n", result.Description)
		if result.NewClue != "" {
			fmt.Printf("发现线索: %s\n", result.NewClue)
		}
		if result.FoundItem != nil {
			fmt.Printf("发现物品: %s (%s)\n", result.FoundItem.Name, result.FoundItem.Type)
		}
		if result.StoryNode != nil {
			fmt.Printf("触发故事节点: %s...\n", result.StoryNode.Content[:min(50, len(result.StoryNode.Content))])
		}
	} else {
		fmt.Println("⚠️  未生成探索结果")
	}
}

// 5. 配置LLM
func configureLLM() {
	fmt.Println(T("llm_config"))
	fmt.Println()

	// 从配置文件加载现有配置
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		fmt.Println("❌ 配置未加载")
		return
	}

	hasAPIKey := printLLMConfigStatus(cfg)

	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  1) 交互式配置 (Interactive Config)")
	fmt.Println("  2) 从 config.json 重载 (Reload from config.json)")
	fmt.Println("  0) 返回 (Return)")

	choice := getUserInput("请选择: ")
	if choice == "2" {
		if _, err := config.Load(); err != nil {
			fmt.Printf("❌ 加载配置失败: %v\n", err)
			return
		}
		// 重新初始化LLM服务
		if err := app.ReinitializeLLMService(); err != nil {
			fmt.Printf("⚠️  LLM服务重初始化失败: %v\n", err)
		} else {
			fmt.Println("✅ 配置已重载，服务已更新")
			if updatedCfg := config.GetCurrentConfig(); updatedCfg != nil {
				cfg = updatedCfg
			}
			printLLMConfigStatus(cfg)
		}
		return
	} else if choice == "0" {
		return
	}

	fmt.Println()
	fmt.Println("支持的LLM提供商:")
	fmt.Println("  - openai (OpenAI GPT系列)")
	fmt.Println("  - anthropic (Claude系列)")
	fmt.Println("  - qwen (通义千问)")
	fmt.Println("  - deepseek (DeepSeek)")
	fmt.Println("  - glm (智谱AI)")
	fmt.Println("  - google (Gemini)")
	fmt.Println("  - mistral (Mistral AI)")
	fmt.Println("  - grok (xAI Grok)")
	fmt.Println("  - githubmodels (GitHub Models)")
	fmt.Println("  - openrouter (OpenRouter)")
	fmt.Println()

	currentProvider := cfg.LLMProvider
	if currentProvider == "" {
		currentProvider = "qwen" // 默认提供商
	}
	provider := getUserInputWithDefault("LLM 提供商", currentProvider)

	model := cfg.LLMConfig["default_model"]
	if model == "" {
		// 根据提供商设置默认模型
		defaultModels := map[string]string{
			"openai":       "gpt-4o",
			"anthropic":    "claude-3-5-sonnet-20241022",
			"qwen":         "qwen3-max",
			"deepseek":     "deepseek-chat",
			"glm":          "glm-4-plus",
			"google":       "gemini-2.5-flash",
			"mistral":      "mistral-large-latest",
			"grok":         "grok3",
			"githubmodels": "gpt-4o",
			"openrouter":   "anthropic/claude-3.5-sonnet",
		}
		if defaultModel, exists := defaultModels[provider]; exists {
			model = defaultModel
		} else {
			model = "gpt-4o"
		}
	}
	newModel := getUserInputWithDefault("模型名称", model)

	// 处理API密钥
	var apiKey string
	if hasAPIKey {
		fmt.Println()
		fmt.Println("当前已有API密钥配置")
		updateKey := getUserInputWithDefault("是否更新API密钥? (y/N)", "n")
		if strings.ToLower(updateKey) == "y" || strings.ToLower(updateKey) == "yes" {
			apiKey = getUserInput("请输入新的API密钥: ")
		} else {
			// 保持原有密钥
			apiKey = cfg.LLMConfig["api_key"]
		}
	} else {
		fmt.Println()
		apiKey = getUserInput("请输入API密钥: ")
	}

	if apiKey == "" {
		fmt.Println("❌ API密钥不能为空")
		return
	}

	llmConfig := make(map[string]string)
	llmConfig["default_model"] = newModel
	llmConfig["api_key"] = apiKey

	fmt.Println()
	fmt.Println("正在保存配置...")
	if err := config.UpdateLLMConfig(provider, llmConfig); err != nil {
		fmt.Printf("❌ 配置LLM失败: %v\n", err)
		return
	}

	// 重新初始化LLM服务以应用新配置
	fmt.Println("正在初始化LLM服务...")
	if err := app.ReinitializeLLMService(); err != nil {
		fmt.Printf("⚠️  LLM服务重初始化失败: %v\n", err)
		fmt.Println("⚠️  某些功能可能仍不可用，建议重启应用")
	} else {
		fmt.Println("🔄 LLM服务已成功初始化")
	}

	fmt.Println()
	fmt.Println("✅ LLM配置成功！")
	fmt.Printf("   提供商: %s\n", provider)
	fmt.Printf("   模型: %s\n", newModel)
	fmt.Println("   API密钥: 已配置 ✓")
}

func printLLMConfigStatus(cfg *config.AppConfig) bool {
	fmt.Println("当前配置状态:")
	if cfg == nil {
		fmt.Println("  提供商: 未配置")
		fmt.Println("  API密钥: 未配置 ✗")
		return false
	}

	if cfg.LLMProvider != "" {
		fmt.Printf("  提供商: %s\n", cfg.LLMProvider)
	} else {
		fmt.Println("  提供商: 未配置")
	}

	hasAPIKey := cfg.LLMConfig != nil && cfg.LLMConfig["api_key"] != ""
	if hasAPIKey {
		fmt.Println("  API密钥: 已配置 ✓")
	} else {
		fmt.Println("  API密钥: 未配置 ✗")
	}

	return hasAPIKey
}

// 辅助函数：确保API密钥不会被截断
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 6. 显示当前服务状态
func displayServiceStatus() {
	fmt.Println("📊 当前服务状态:")
	fmt.Println()

	// 显示配置信息
	cfg := config.GetCurrentConfig()
	if cfg != nil {
		fmt.Println("系统配置:")
		fmt.Printf("  服务端口: %s\n", cfg.Port)
		fmt.Printf("  数据目录: %s\n", cfg.DataDir)
		fmt.Printf("  静态文件目录: %s\n", cfg.StaticDir)
		fmt.Printf("  日志目录: %s\n", cfg.LogDir)
		fmt.Printf("  调试模式: %t\n", cfg.DebugMode)
		fmt.Println()

		// 显示LLM配置状态
		fmt.Println("LLM 服务配置:")
		if cfg.LLMProvider != "" {
			fmt.Printf("  提供商: %s\n", cfg.LLMProvider)
		} else {
			fmt.Println("  提供商: 未配置 ✗")
		}

		if cfg.LLMConfig != nil {
			if model := cfg.LLMConfig["default_model"]; model != "" {
				fmt.Printf("  默认模型: %s\n", model)
			}
			if cfg.LLMConfig["api_key"] != "" {
				fmt.Println("  API密钥: 已配置 ✓")
			} else {
				fmt.Println("  API密钥: 未配置 ✗")
			}
		} else {
			fmt.Println("  配置: 未初始化 ✗")
		}
	} else {
		fmt.Println("配置: 未初始化")
	}

	fmt.Println()

	// 检查依赖注入容器中注册的服务
	container := di.GetContainer()
	if container != nil {
		serviceNames := container.GetNames()
		fmt.Printf("已注册服务数量: %d\n", len(serviceNames))

		// 检查LLM服务状态
		if llmService, ok := container.Get("llm").(*services.LLMService); ok && llmService != nil {
			fmt.Println()
			fmt.Println("LLM 服务状态:")
			if llmService.IsReady() {
				fmt.Println("  状态: 就绪 ✓")
				fmt.Printf("  提供商: %s\n", llmService.GetProviderName())
			} else {
				fmt.Println("  状态: 未就绪 ✗")
				fmt.Printf("  原因: %s\n", llmService.GetReadyState())
			}
		}

		if len(serviceNames) > 0 {
			fmt.Println()
			fmt.Println("已注册的服务:")
			for _, name := range serviceNames {
				fmt.Printf("  - %s\n", name)
			}
		}
	} else {
		fmt.Println("依赖注入容器: 未初始化")
	}
}

// 查看当前配置
func viewConfig() {
	fmt.Println("⚙️  当前配置信息:")
	cfg := config.GetCurrentConfig()
	if cfg == nil {
		fmt.Println("  配置未初始化")
		return
	}

	fmt.Printf("  端口: %s\n", cfg.Port)
	fmt.Printf("  数据目录: %s\n", cfg.DataDir)
	fmt.Printf("  静态文件目录: %s\n", cfg.StaticDir)
	fmt.Printf("  模板目录: %s\n", cfg.TemplatesDir)
	fmt.Printf("  日志目录: %s\n", cfg.LogDir)
	fmt.Printf("  调试模式: %t\n", cfg.DebugMode)
	fmt.Printf("  LLM 提供商: %s\n", cfg.LLMProvider)

	if cfg.LLMConfig != nil {
		fmt.Println("  LLM 配置:")
		for k, v := range cfg.LLMConfig {
			if k == "api_key" {
				fmt.Printf("    %s: [已配置但已隐藏]\n", k)
			} else {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	} else {
		fmt.Println("  LLM 配置: 未设置")
	}
}

// 7. 列出所有服务
func listServices() {
	fmt.Println("📦 已注册的服务:")
	container := di.GetContainer()
	if container == nil {
		fmt.Println("  依赖注入容器未初始化")
		return
	}

	serviceNames := container.GetNames()
	if len(serviceNames) == 0 {
		fmt.Println("  暂无注册的服务")
		return
	}

	for _, name := range serviceNames {
		service := container.Get(name)
		if service != nil {
			fmt.Printf("  - %s (%T)\n", name, service)
		} else {
			fmt.Printf("  - %s (nil)\n", name)
		}
	}
}

// 物品管理功能
func manageItems() {
	fmt.Println("🎒 管理物品")
	container := di.GetContainer()
	itemService := container.Get("item").(*services.ItemService)
	sceneService := container.Get("scene").(*services.SceneService)

	if itemService == nil {
		fmt.Println("❌ 物品服务未初始化")
		return
	}

	if sceneService == nil {
		fmt.Println("❌ 场景服务未初始化")
		return
	}

	fmt.Println("物品功能菜单:")
	fmt.Println("  l) 列出所有物品")
	fmt.Println("  c) 创建新物品")
	fmt.Println("  v) 查看物品详情")
	fmt.Println("  u) 更新物品")
	fmt.Println("  d) 删除物品")
	fmt.Println("  b) 返回主菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		items, err := itemService.GetAllItems(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取物品失败: %v\n", err)
			return
		}

		if len(items) == 0 {
			fmt.Println("当前场景中没有物品")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个物品:\n", sceneID, len(items))
		for i, item := range items {
			fmt.Printf("  %d) %s (%s) - %s\n", i+1, item.Name, item.Type, item.Description)
		}
	case "c":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		name := getUserInput("物品名称: ")
		description := getUserInput("物品描述: ")
		location := getUserInputWithDefault("位置 (默认: unknown): ", "unknown")
		imageURL := getUserInputWithDefault("图片URL (可选): ", "")
		itemType := getUserInputWithDefault("物品类型 (默认: Unknown): ", "Unknown")
		usableWith := getUserInput("可使用对象 (可选，多个用逗号分隔): ")
		isOwnedInput := getUserInputWithDefault("是否拥有 (y/N): ", "n")

		// 解析可使用对象
		var usableWithList []string
		if usableWith != "" {
			usableWithList = strings.Split(usableWith, ",")
			// 去除空格
			for i, item := range usableWithList {
				usableWithList[i] = strings.TrimSpace(item)
			}
		}

		// 解析是否拥有
		isOwned := strings.ToLower(isOwnedInput) == "y" || strings.ToLower(isOwnedInput) == "yes"

		item := models.Item{
			ID:          fmt.Sprintf("item_%d", time.Now().UnixNano()),
			SceneID:     sceneID,
			Name:        name,
			Description: description,
			Location:    location,
			ImageURL:    imageURL,
			Type:        itemType,
			Properties:  make(map[string]any), // Properties 是 map[string]any 类型
			UsableWith:  usableWithList,
			IsOwned:     isOwned,
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
		}

		// 为Properties添加一些基本属性
		propertiesInput := getUserInput("物品属性 (JSON格式，可选): ")
		if propertiesInput != "" {
			// 简单处理：将整个输入字符串作为"custom"属性值
			item.Properties["custom"] = propertiesInput
		}

		if err := itemService.AddItem(sceneID, &item); err != nil {
			fmt.Printf("❌ 添加物品失败: %v\n", err)
		} else {
			fmt.Printf("✅ 物品 '%s' 添加成功！物品ID: %s\n", item.Name, item.ID)
		}
	case "v":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		items, err := itemService.GetAllItems(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取物品失败: %v\n", err)
			return
		}

		if len(items) == 0 {
			fmt.Println("当前场景中没有物品")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个物品:\n", sceneID, len(items))
		for i, item := range items {
			fmt.Printf("  %d) %s (%s)\n", i+1, item.Name, item.Type)
		}

		itemNum := getUserInput("输入要查看的物品编号: ")
		if itemNum == "" {
			return
		}

		// 解析物品编号
		itemIndex := 0
		if _, err := fmt.Sscanf(itemNum, "%d", &itemIndex); err != nil {
			fmt.Println("❌ 无效的物品编号")
			return
		}
		itemIndex-- // 转换为0基索引

		if itemIndex < 0 || itemIndex >= len(items) {
			fmt.Println("❌ 物品编号超出范围")
			return
		}

		selectedItem := items[itemIndex]
		fmt.Printf("\n=== 物品详情 ===\n")
		fmt.Printf("ID: %s\n", selectedItem.ID)
		fmt.Printf("场景ID: %s\n", selectedItem.SceneID)
		fmt.Printf("名称: %s\n", selectedItem.Name)
		fmt.Printf("类型: %s\n", selectedItem.Type)
		fmt.Printf("描述: %s\n", selectedItem.Description)
		fmt.Printf("位置: %s\n", selectedItem.Location)
		fmt.Printf("图片URL: %s\n", selectedItem.ImageURL)
		fmt.Printf("是否拥有: %t\n", selectedItem.IsOwned)
		fmt.Printf("可使用对象: %s\n", strings.Join(selectedItem.UsableWith, ", "))
		fmt.Printf("属性: %v\n", selectedItem.Properties)
		fmt.Printf("创建时间: %s\n", selectedItem.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("最后更新: %s\n", selectedItem.LastUpdated.Format("2006-01-02 15:04:05"))
	case "u":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		items, err := itemService.GetAllItems(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取物品失败: %v\n", err)
			return
		}

		if len(items) == 0 {
			fmt.Println("当前场景中没有物品")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个物品:\n", sceneID, len(items))
		for i, item := range items {
			fmt.Printf("  %d) %s (%s)\n", i+1, item.Name, item.Type)
		}

		itemNum := getUserInput("输入要更新的物品编号: ")
		if itemNum == "" {
			return
		}

		// 解析物品编号
		itemIndex := 0
		if _, err := fmt.Sscanf(itemNum, "%d", &itemIndex); err != nil {
			fmt.Println("❌ 无效的物品编号")
			return
		}
		itemIndex-- // 转换为0基索引

		if itemIndex < 0 || itemIndex >= len(items) {
			fmt.Println("❌ 物品编号超出范围")
			return
		}

		selectedItem := items[itemIndex]
		fmt.Printf("正在更新物品 '%s' (ID: %s)\n", selectedItem.Name, selectedItem.ID)

		name := getUserInputWithDefault("物品名称 (当前: "+selectedItem.Name+"): ", selectedItem.Name)
		description := getUserInputWithDefault("物品描述 (当前: "+selectedItem.Description+"): ", selectedItem.Description)
		location := getUserInputWithDefault("位置 (当前: "+selectedItem.Location+"): ", selectedItem.Location)
		imageURL := getUserInputWithDefault("图片URL (当前: "+selectedItem.ImageURL+"): ", selectedItem.ImageURL)
		itemType := getUserInputWithDefault("物品类型 (当前: "+selectedItem.Type+"): ", selectedItem.Type)
		usableWith := getUserInputWithDefault("可使用对象 (当前: "+strings.Join(selectedItem.UsableWith, ", ")+"): ", strings.Join(selectedItem.UsableWith, ", "))
		isOwnedInput := getUserInputWithDefault("是否拥有 (当前: "+fmt.Sprintf("%t", selectedItem.IsOwned)+"): ", fmt.Sprintf("%t", selectedItem.IsOwned))

		// 解析可使用对象
		var usableWithList []string
		if usableWith != "" {
			usableWithList = strings.Split(usableWith, ",")
			// 去除空格
			for i, item := range usableWithList {
				usableWithList[i] = strings.TrimSpace(item)
			}
		}

		// 解析是否拥有
		isOwned := strings.ToLower(isOwnedInput) == "true" || strings.ToLower(isOwnedInput) == "t" || strings.ToLower(isOwnedInput) == "1"

		updatedItem := models.Item{
			ID:          selectedItem.ID,
			SceneID:     selectedItem.SceneID,
			Name:        name,
			Description: description,
			Location:    location,
			ImageURL:    imageURL,
			Type:        itemType,
			Properties:  selectedItem.Properties, // 保持原有的属性
			UsableWith:  usableWithList,
			IsOwned:     isOwned,
			CreatedAt:   selectedItem.CreatedAt, // 保留原始创建时间
			LastUpdated: time.Now(),
		}

		// 为Properties添加一些基本属性
		propertiesInput := getUserInputWithDefault("物品属性 (JSON格式，当前: custom="+fmt.Sprintf("%v", selectedItem.Properties["custom"])+"): ", fmt.Sprintf("%v", selectedItem.Properties["custom"]))
		if propertiesInput != "" {
			// 更新custom属性
			updatedItem.Properties["custom"] = propertiesInput
		} else {
			// 如果用户输入为空，保留现有的属性
			updatedItem.Properties = selectedItem.Properties
		}

		if err := itemService.UpdateItem(sceneID, &updatedItem); err != nil {
			fmt.Printf("❌ 更新物品失败: %v\n", err)
		} else {
			fmt.Printf("✅ 物品 '%s' 更新成功！\n", updatedItem.Name)
		}
	case "d":
		sceneID := getUserInput("请输入场景ID: ")
		if sceneID == "" {
			fmt.Println("❌ 场景ID不能为空")
			return
		}

		items, err := itemService.GetAllItems(sceneID)
		if err != nil {
			fmt.Printf("❌ 读取物品失败: %v\n", err)
			return
		}

		if len(items) == 0 {
			fmt.Println("当前场景中没有物品")
			return
		}

		fmt.Printf("场景 '%s' 共有 %d 个物品:\n", sceneID, len(items))
		for i, item := range items {
			fmt.Printf("  %d) %s (%s)\n", i+1, item.Name, item.Type)
		}

		itemNum := getUserInput("输入要删除的物品编号: ")
		if itemNum == "" {
			return
		}

		// 解析物品编号
		itemIndex := 0
		if _, err := fmt.Sscanf(itemNum, "%d", &itemIndex); err != nil {
			fmt.Println("❌ 无效的物品编号")
			return
		}
		itemIndex-- // 转换为0基索引

		if itemIndex < 0 || itemIndex >= len(items) {
			fmt.Println("❌ 物品编号超出范围")
			return
		}

		itemToDelete := items[itemIndex]
		confirm := getUserInput(fmt.Sprintf("确认删除物品 '%s' (y/N): ", itemToDelete.Name))
		if strings.ToLower(confirm) == "y" || strings.ToLower(confirm) == "yes" {
			if err := itemService.DeleteItem(sceneID, itemToDelete.ID); err != nil {
				fmt.Printf("❌ 删除物品失败: %v\n", err)
			} else {
				fmt.Printf("✅ 物品 '%s' 删除成功！\n", itemToDelete.Name)
			}
		} else {
			fmt.Println("❌ 删除操作已取消")
		}
	case "b":
		fmt.Println("🔙 返回主菜单")
		return
	}
}

// 7. 与场景互动
func interactWithScene() {
	fmt.Println(T("interact_title"))
	container := di.GetContainer()

	sceneService, _ := container.Get("scene").(*services.SceneService)
	storyService, _ := container.Get("story").(*services.StoryService)
	llmProvider := container.Get("llm")
	itemService, _ := container.Get("item").(*services.ItemService)
	userService, _ := container.Get("user").(*services.UserService)

	if sceneService == nil || storyService == nil || llmProvider == nil {
		fmt.Println("❌ 服务未完全初始化")
		return
	}
	if itemService == nil || userService == nil {
		fmt.Println("❌ 用户或物品服务未初始化")
		return
	}

	sceneID := getUserInput(T("enter_scene_id"))
	if sceneID == "" {
		fmt.Println(T("scene_id_empty"))
		return
	}

	sceneData, err := sceneService.LoadScene(sceneID)
	if err != nil {
		fmt.Printf("❌ 指定场景不存在: %v\n", err)
		return
	}

	userPrefs := getConsoleUserPreferences(userService)
	storyData, err := ensureStoryPrepared(sceneID, storyService, userPrefs)
	if err != nil {
		fmt.Printf("❌ 初始化故事失败: %v\n", err)
		return
	}

	autoPushed := autoAdvanceFirstNode(sceneID, storyService, userPrefs, storyData)
	hudDirty := true
	if autoPushed {
		hudDirty = true
	}

	fmt.Printf(T("interact_scene_banner")+"\n", sceneData.Scene.Title)
	fmt.Println(T("interact_help"))

	scanner := bufio.NewScanner(os.Stdin)
	llmService, _ := llmProvider.(*services.LLMService)
	if llmService == nil {
		fmt.Println("❌ 无法获取LLM服务实例")
		return
	}

	lastNodeStamp := ""
	for {
		storyData, err = storyService.GetStoryForScene(sceneID)
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}

		characters, _ := sceneService.GetCharactersByScene(sceneID)
		items, _ := itemService.GetAllItems(sceneID)
		skills, _ := userService.GetUserSkills(defaultConsoleUserID)

		if hudDirty {
			lastNodeStamp = displayLatestStoryNode(storyData, lastNodeStamp)
			renderInteractionHUD(sceneData.Scene.Title, storyData, characters, items, skills)
			hudDirty = false
		}

		fmt.Print(T("user_input"))
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if strings.EqualFold(input, "quit") || strings.EqualFold(input, "exit") {
			break
		}

		if strings.HasPrefix(input, "!") {
			handled, exitApp, refresh := handleSystemCommand(input, sceneID, storyService, storyData, userPrefs)
			if exitApp {
				return
			}
			if handled {
				hudDirty = hudDirty || refresh
				continue
			}
		}

		if input == "" {
			prefCopy := *userPrefs
			update, err := storyService.AdvanceStory(sceneID, &prefCopy)
			if err != nil {
				fmt.Printf("❌ 推进失败: %v\n", err)
			} else {
				presentStoryUpdate(update)
			}
			hudDirty = true
			continue
		}

		var contextNote string
		if strings.HasPrefix(input, "@") {
			input, contextNote = enrichMentionInput(input, characters, storyData.Locations)
		} else if strings.HasPrefix(input, "/") {
			input, contextNote = enrichSlashInput(input, items, skills)
		}

		if contextNote != "" {
			printBox(T("context_box_title"), contextNote)
		}

		prompt := fmt.Sprintf(`场景: %s
场景描述: %s

用户输入: %s

请根据场景背景和用户输入提供适当的回应。`,
			sceneData.Scene.Title,
			sceneData.Scene.Description,
			input)

		if contextNote != "" {
			prompt += "\n\n" + T("context_prompt_hint") + " " + contextNote
		}

		resp, err := llmService.CreateChatCompletion(
			context.Background(),
			services.ChatCompletionRequest{
				Model: userPrefs.PreferredModel,
				Messages: []services.ChatCompletionMessage{
					{Role: "system", Content: "你是一个故事助手，根据场景背景和用户输入提供适当的回应。"},
					{Role: "user", Content: prompt},
				},
				MaxTokens: 500,
			},
		)

		if err != nil {
			fmt.Printf("❌ AI响应生成失败: %v\n", err)
			continue
		}

		if len(resp.Choices) > 0 {
			responseText := resp.Choices[0].Message.Content
			printBox(T("ai_response"), responseText)
			hudDirty = true
		} else {
			fmt.Println("❌ 未收到AI响应")
		}
	}

	fmt.Println(T("interaction_end"))
}

// --- 新增功能 ---

var currentLanguage = "zh"

var translations = map[string]map[string]string{
	"zh": {
		"menu_title":              "请选择功能:",
		"menu_llm":                "1) 配置LLM (大语言模型)",
		"menu_scenes":             "2) 管理场景 (Scenes)",
		"menu_characters":         "3) 管理角色 (Characters)",
		"menu_stories":            "4) 管理故事 (Stories)",
		"menu_items":              "5) 管理物品 (Items)",
		"menu_skills":             "6) 管理技能 (Skills)",
		"menu_interact":           "7) 与场景互动",
		"menu_export":             "8) 导出故事",
		"menu_config":             "9) 查看当前配置",
		"menu_status":             "10) 显示当前服务状态",
		"menu_services":           "11) 列出所有服务",
		"menu_exit":               "0) 退出",
		"input_prompt":            "请选择操作 (输入数字或命令): ",
		"invalid_choice":          "❌ 无效选择，请重新输入！",
		"goodbye":                 "👋 感谢使用 SceneIntruderMCP 控制台应用程序！",
		"init_success":            "✅ 项目环境初始化成功！",
		"scene_manage":            "🎬 管理场景",
		"char_manage":             "👤 管理角色",
		"story_manage":            "📚 管理故事",
		"item_manage":             "🎒 管理物品",
		"skill_manage":            "⚡ 管理技能",
		"interact_title":          "💬 与场景互动",
		"export_title":            "📤 导出故事",
		"llm_config":              "🤖 配置LLM",
		"status_title":            "📊 当前服务状态",
		"services_list":           "📦 已注册的服务",
		"config_view":             "⚙️  当前配置信息",
		"enter_scene_id":          "请输入场景ID: ",
		"scene_id_empty":          "❌ 场景ID不能为空",
		"read_fail":               "❌ 读取失败: %v",
		"create_success":          "✅ 创建成功！",
		"update_success":          "✅ 更新成功！",
		"delete_success":          "✅ 删除成功！",
		"op_cancel":               "❌ 操作已取消",
		"confirm_delete":          "确认删除 '%s' (y/N): ",
		"return_menu":             "🔙 返回主菜单",
		"interact_help":           "输入 'quit' 退出, '@' 呼叫角色/地点, '/' 使用物品/技能, '!' 调用系统指令",
		"ai_response":             "🤖 AI响应",
		"user_input":              "输入您的问题或指令: ",
		"interact_scene_banner":   "正在与场景 '%s' 互动...",
		"context_box_title":       "🎯 上下文提示",
		"context_prompt_hint":     "请结合这些上下文生成更贴合的回应:",
		"interaction_end":         "结束与场景互动",
		"auto_push_notice":        "⚡ 正在自动推进首个故事节点...",
		"auto_push_failed":        "自动推进失败",
		"hud_header":              "场景摘要",
		"hud_progress":            "进度",
		"hud_state":               "状态",
		"hud_nodes":               "节点数",
		"hud_tasks":               "任务数",
		"hud_locations":           "地点数",
		"panel_story_node":        "📖 最新故事节点",
		"panel_characters":        "👤 角色 (@)",
		"panel_locations":         "📍 地点 (@)",
		"panel_items":             "🎒 物品 (/)",
		"panel_skills":            "⚡ 技能 (/)",
		"panel_system":            "🛠 系统指令 (!)",
		"panel_empty":             "暂无可用数据",
		"panel_more":              "还有 %d 项...",
		"panel_choices":           "可选项:",
		"choice_pending":          "待选择",
		"choice_selected":         "已选择",
		"cmd_status_desc":         "查看故事进度",
		"cmd_tasks_desc":          "查看任务列表",
		"cmd_nodes_desc":          "列出所有节点",
		"cmd_advance_desc":        "立即推进故事",
		"cmd_help_desc":           "重新显示帮助",
		"cmd_menu_desc":           "返回主菜单",
		"cmd_status_title":        "📊 故事状态",
		"cmd_tasks_title":         "🗒 任务列表",
		"cmd_nodes_title":         "🌲 节点概览",
		"cmd_help_title":          "ℹ️ 指令帮助",
		"cmd_unknown":             "⚠️ 未识别的系统指令",
		"no_story_update":         "⚠️ 暂无新的故事事件",
		"cmd_new_task":            "新任务",
		"cmd_new_clue":            "新线索",
		"context_character":       "@%s · %s\n%s",
		"context_character_input": "与 %s 互动: %s",
		"context_location":        "@%s · %s",
		"context_location_input":  "前往 %s: %s",
		"context_item":            "/%s · %s",
		"context_item_input":      "使用物品 %s: %s",
		"context_skill":           "/%s · %s",
		"context_skill_input":     "激活技能 %s: %s",
		"task_pending":            "进行中",
		"task_done":               "已完成",
		"node_hidden":             "隐藏",
		"node_shown":              "已显示",
	},
	"en": {
		"menu_title":              "Please select a function:",
		"menu_llm":                "1) Configure LLM",
		"menu_scenes":             "2) Manage Scenes",
		"menu_characters":         "3) Manage Characters",
		"menu_stories":            "4) Manage Stories",
		"menu_items":              "5) Manage Items",
		"menu_skills":             "6) Manage Skills",
		"menu_interact":           "7) Interact with Scene",
		"menu_export":             "8) Export Story",
		"menu_config":             "9) View Configuration",
		"menu_status":             "10) Show Service Status",
		"menu_services":           "11) List All Services",
		"menu_exit":               "0) Exit",
		"input_prompt":            "Select operation (number or command): ",
		"invalid_choice":          "❌ Invalid choice, please try again!",
		"goodbye":                 "👋 Thank you for using SceneIntruderMCP Console App!",
		"init_success":            "✅ Project environment initialized successfully!",
		"scene_manage":            "🎬 Manage Scenes",
		"char_manage":             "👤 Manage Characters",
		"story_manage":            "📚 Manage Stories",
		"item_manage":             "🎒 Manage Items",
		"skill_manage":            "⚡ Manage Skills",
		"interact_title":          "💬 Interact with Scene",
		"export_title":            "📤 Export Story",
		"llm_config":              "🤖 Configure LLM",
		"status_title":            "📊 Current Service Status",
		"services_list":           "📦 Registered Services",
		"config_view":             "⚙️  Current Configuration",
		"enter_scene_id":          "Enter Scene ID: ",
		"scene_id_empty":          "❌ Scene ID cannot be empty",
		"read_fail":               "❌ Read failed: %v",
		"create_success":          "✅ Created successfully!",
		"update_success":          "✅ Updated successfully!",
		"delete_success":          "✅ Deleted successfully!",
		"op_cancel":               "❌ Operation cancelled",
		"confirm_delete":          "Confirm delete '%s' (y/N): ",
		"return_menu":             "🔙 Return to Main Menu",
		"interact_help":           "Type 'quit' to exit, '@' for characters/locations, '/' for items/skills, '!' for system commands",
		"ai_response":             "🤖 AI Response",
		"user_input":              "Enter your input or command: ",
		"interact_scene_banner":   "Interacting with scene '%s'...",
		"context_box_title":       "🎯 Context",
		"context_prompt_hint":     "Use this context to enrich the reply:",
		"interaction_end":         "Interaction finished",
		"auto_push_notice":        "⚡ Auto-advancing the first story beat...",
		"auto_push_failed":        "auto advance failed",
		"hud_header":              "Scene Snapshot",
		"hud_progress":            "Progress",
		"hud_state":               "State",
		"hud_nodes":               "Nodes",
		"hud_tasks":               "Tasks",
		"hud_locations":           "Locations",
		"panel_story_node":        "📖 Latest Story Node",
		"panel_characters":        "👤 Characters (@)",
		"panel_locations":         "📍 Locations (@)",
		"panel_items":             "🎒 Items (/)",
		"panel_skills":            "⚡ Skills (/)",
		"panel_system":            "🛠 System Commands (!)",
		"panel_empty":             "No entries yet",
		"panel_more":              "...and %d more",
		"panel_choices":           "Choices:",
		"choice_pending":          "pending",
		"choice_selected":         "selected",
		"cmd_status_desc":         "View story status",
		"cmd_tasks_desc":          "Show tasks",
		"cmd_nodes_desc":          "List nodes",
		"cmd_advance_desc":        "Advance story now",
		"cmd_help_desc":           "Show help",
		"cmd_menu_desc":           "Return to menu",
		"cmd_status_title":        "📊 Story Status",
		"cmd_tasks_title":         "🗒 Tasks",
		"cmd_nodes_title":         "🌲 Nodes",
		"cmd_help_title":          "ℹ️ Help",
		"cmd_unknown":             "⚠️ Unknown system command",
		"no_story_update":         "⚠️ No new story event",
		"cmd_new_task":            "New Task",
		"cmd_new_clue":            "New Clue",
		"context_character":       "@%s · %s\n%s",
		"context_character_input": "Interact with %s: %s",
		"context_location":        "@%s · %s",
		"context_location_input":  "Travel to %s: %s",
		"context_item":            "/%s · %s",
		"context_item_input":      "Use item %s: %s",
		"context_skill":           "/%s · %s",
		"context_skill_input":     "Activate skill %s: %s",
		"task_pending":            "active",
		"task_done":               "completed",
		"node_hidden":             "hidden",
		"node_shown":              "revealed",
	},
}

func T(key string, args ...interface{}) string {
	langMap, ok := translations[currentLanguage]
	if !ok {
		langMap = translations["zh"]
	}
	val, ok := langMap[key]
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(val, args...)
	}
	return val
}

func selectLanguage() {
	fmt.Println("Select Language / 选择语言:")
	fmt.Println("  1) English")
	fmt.Println("  2) 中文 (Chinese)")
	choice := getUserInput("Choice/选择 [2]: ")
	if choice == "1" {
		currentLanguage = "en"
	} else {
		currentLanguage = "zh"
	}
	fmt.Printf("Language set to %s\n\n", currentLanguage)
}

const (
	cliBoxMaxWidth = 90
	hudMaxEntries  = 5
)

func printBox(title, content string) {
	wrappedLines := wrapContentForBox(content, cliBoxMaxWidth)
	maxWidth := utf8.RuneCountInString(title)
	for _, line := range wrappedLines {
		if w := utf8.RuneCountInString(line); w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth < 0 {
		maxWidth = 0
	}
	border := strings.Repeat("─", maxWidth+2)
	fmt.Println("┌" + border + "┐")
	if title != "" {
		fmt.Printf("│ %s │\n", padRight(title, maxWidth))
		fmt.Println("├" + border + "┤")
	}
	if len(wrappedLines) == 0 {
		wrappedLines = []string{""}
	}
	for _, line := range wrappedLines {
		fmt.Printf("│ %s │\n", padRight(line, maxWidth))
	}
	fmt.Println("└" + border + "┘")
}

func wrapContentForBox(content string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{content}
	}
	var result []string
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimRight(rawLine, " ")
		runes := []rune(line)
		for len(runes) > maxWidth {
			result = append(result, string(runes[:maxWidth]))
			runes = runes[maxWidth:]
		}
		result = append(result, string(runes))
	}
	return result
}

func padRight(text string, width int) string {
	current := utf8.RuneCountInString(text)
	if current >= width {
		return text
	}
	return text + strings.Repeat(" ", width-current)
}

func getConsoleUserPreferences(userService *services.UserService) *models.UserPreferences {
	if userService != nil {
		if prefs, err := userService.GetUserPreferences(defaultConsoleUserID); err == nil {
			if prefs.PreferredModel == "" {
				prefs.PreferredModel = "qwen3-max"
			}
			prefs.AllowPlotTwists = true
			copyPrefs := prefs
			return &copyPrefs
		}
	}
	return &models.UserPreferences{
		PreferredModel:  "qwen3-max",
		CreativityLevel: models.CreativityBalanced,
		AllowPlotTwists: true,
	}
}

func ensureStoryPrepared(sceneID string, storyService *services.StoryService, prefs *models.UserPreferences) (*models.StoryData, error) {
	storyData, err := storyService.GetStoryForScene(sceneID)
	if err == nil {
		return storyData, nil
	}

	if !strings.Contains(err.Error(), "故事数据不存在") && !strings.Contains(strings.ToLower(err.Error()), "story data does not exist") {
		return nil, err
	}

	prefCopy := *prefs
	if _, initErr := storyService.InitializeStoryForScene(sceneID, &prefCopy); initErr != nil {
		return nil, initErr
	}

	return storyService.GetStoryForScene(sceneID)
}

func autoAdvanceFirstNode(sceneID string, storyService *services.StoryService, prefs *models.UserPreferences, storyData *models.StoryData) bool {
	if storyData == nil || storyData.Progress > 0 {
		return false
	}
	fmt.Println(T("auto_push_notice"))
	prefCopy := *prefs
	update, err := storyService.AdvanceStory(sceneID, &prefCopy)
	if err != nil {
		fmt.Printf("⚠️  %s: %v\n", T("auto_push_failed"), err)
		return false
	}
	presentStoryUpdate(update)
	return true
}

func displayLatestStoryNode(storyData *models.StoryData, lastStamp string) string {
	if storyData == nil {
		return lastStamp
	}
	var latest *models.StoryNode
	for i := range storyData.Nodes {
		node := &storyData.Nodes[i]
		if node.IsRevealed {
			if latest == nil || node.CreatedAt.After(latest.CreatedAt) {
				latest = node
			}
		}
	}
	if latest == nil {
		return lastStamp
	}
	stamp := fmt.Sprintf("%s-%d", latest.ID, latest.CreatedAt.UnixNano())
	if stamp == lastStamp {
		return lastStamp
	}
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s\n\n%s\n", truncateForCLI(latest.ID, 28), latest.Content))
	if len(latest.Choices) > 0 {
		builder.WriteString(T("panel_choices") + "\n")
		for _, choice := range latest.Choices {
			status := T("choice_pending")
			if choice.Selected {
				status = T("choice_selected")
			}
			builder.WriteString(fmt.Sprintf("- %s (%s)\n", choice.Text, status))
		}
	}
	printBox(T("panel_story_node"), strings.TrimRight(builder.String(), "\n"))
	return stamp
}

func renderInteractionHUD(sceneTitle string, storyData *models.StoryData, characters []*models.Character, items []*models.Item, skills []models.UserSkill) {
	if storyData == nil {
		return
	}
	summary := fmt.Sprintf("%s\n%s: %d%% · %s", sceneTitle, T("hud_progress"), storyData.Progress, storyData.CurrentState)
	printBox(T("hud_header"), summary)

	charLines := make([]string, 0, len(characters))
	for _, c := range characters {
		charLines = append(charLines, fmt.Sprintf("@%s · %s", c.Name, truncateForCLI(c.Role, 20)))
	}
	printBox(T("panel_characters"), formatPanelContent(charLines))

	locationLines := make([]string, 0, len(storyData.Locations))
	for _, loc := range storyData.Locations {
		if loc.Accessible {
			locationLines = append(locationLines, fmt.Sprintf("@%s · %s", loc.Name, truncateForCLI(loc.Description, 32)))
		}
	}
	printBox(T("panel_locations"), formatPanelContent(locationLines))

	itemLines := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsOwned {
			itemLines = append(itemLines, fmt.Sprintf("/%s · %s", item.Name, truncateForCLI(item.Type, 24)))
		}
	}
	printBox(T("panel_items"), formatPanelContent(itemLines))

	skillLines := make([]string, 0, len(skills))
	for _, skill := range skills {
		skillLines = append(skillLines, fmt.Sprintf("/%s · %s", skill.Name, truncateForCLI(skill.Description, 32)))
	}
	printBox(T("panel_skills"), formatPanelContent(skillLines))

	systemLines := []string{
		"!status · " + T("cmd_status_desc"),
		"!tasks · " + T("cmd_tasks_desc"),
		"!nodes · " + T("cmd_nodes_desc"),
		"!advance · " + T("cmd_advance_desc"),
		"!help · " + T("cmd_help_desc"),
		"!menu · " + T("cmd_menu_desc"),
	}
	printBox(T("panel_system"), formatPanelContent(systemLines))
}

func formatPanelContent(lines []string) string {
	if len(lines) == 0 {
		return T("panel_empty")
	}
	if len(lines) <= hudMaxEntries {
		return strings.Join(lines, "\n")
	}
	visible := strings.Join(lines[:hudMaxEntries], "\n")
	return visible + fmt.Sprintf("\n"+T("panel_more"), len(lines)-hudMaxEntries)
}

func handleSystemCommand(input string, sceneID string, storyService *services.StoryService, storyData *models.StoryData, prefs *models.UserPreferences) (handled bool, exitApp bool, refresh bool) {
	cmd := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(input)), "!")
	switch cmd {
	case "status":
		if storyData == nil {
			return true, false, false
		}
		summary := fmt.Sprintf("%s: %d%%\n%s: %s\n%s: %d\n%s: %d\n%s: %d",
			T("hud_progress"), storyData.Progress,
			T("hud_state"), storyData.CurrentState,
			T("hud_nodes"), len(storyData.Nodes),
			T("hud_tasks"), len(storyData.Tasks),
			T("hud_locations"), len(storyData.Locations))
		printBox(T("cmd_status_title"), summary)
		return true, false, false
	case "tasks":
		lines := make([]string, 0, len(storyData.Tasks))
		for _, task := range storyData.Tasks {
			state := T("task_pending")
			if task.Completed {
				state = T("task_done")
			}
			lines = append(lines, fmt.Sprintf("- %s (%s)", task.Title, state))
		}
		printBox(T("cmd_tasks_title"), formatPanelContent(lines))
		return true, false, false
	case "nodes":
		lines := make([]string, 0, len(storyData.Nodes))
		for _, node := range storyData.Nodes {
			state := T("node_hidden")
			if node.IsRevealed {
				state = T("node_shown")
			}
			lines = append(lines, fmt.Sprintf("- %s (%s)", truncateForCLI(node.ID, 32), state))
		}
		printBox(T("cmd_nodes_title"), formatPanelContent(lines))
		return true, false, false
	case "advance":
		prefCopy := *prefs
		update, err := storyService.AdvanceStory(sceneID, &prefCopy)
		if err != nil {
			fmt.Printf("❌ 推进失败: %v\n", err)
		} else {
			presentStoryUpdate(update)
		}
		return true, false, true
	case "help":
		printBox(T("cmd_help_title"), T("interact_help"))
		return true, false, true
	case "menu":
		return true, true, false
	default:
		fmt.Println(T("cmd_unknown"))
		return false, false, false
	}
}

func presentStoryUpdate(update *models.StoryUpdate) {
	if update == nil {
		fmt.Println(T("no_story_update"))
		return
	}
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s\n\n%s", update.Title, update.Content))
	if update.NewTask != nil {
		builder.WriteString(fmt.Sprintf("\n\n%s: %s", T("cmd_new_task"), update.NewTask.Title))
	}
	if update.NewClue != "" {
		builder.WriteString(fmt.Sprintf("\n%s: %s", T("cmd_new_clue"), update.NewClue))
	}
	printBox(T("ai_response"), builder.String())
}

func enrichMentionInput(input string, characters []*models.Character, locations []models.StoryLocation) (string, string) {
	target := strings.TrimSpace(strings.TrimPrefix(input, "@"))
	if target == "" {
		return input, ""
	}
	for _, c := range characters {
		if strings.EqualFold(c.Name, target) {
			note := fmt.Sprintf(T("context_character"), c.Name, truncateForCLI(c.Role, 32), truncateForCLI(c.Description, 120))
			return fmt.Sprintf(T("context_character_input"), c.Name, c.Description), note
		}
	}
	for _, loc := range locations {
		if strings.EqualFold(loc.Name, target) {
			note := fmt.Sprintf(T("context_location"), loc.Name, truncateForCLI(loc.Description, 120))
			return fmt.Sprintf(T("context_location_input"), loc.Name, loc.Description), note
		}
	}
	return input, ""
}

func enrichSlashInput(input string, items []*models.Item, skills []models.UserSkill) (string, string) {
	target := strings.TrimSpace(strings.TrimPrefix(input, "/"))
	if target == "" {
		return input, ""
	}
	for _, item := range items {
		if strings.EqualFold(item.Name, target) {
			note := fmt.Sprintf(T("context_item"), item.Name, truncateForCLI(item.Description, 120))
			return fmt.Sprintf(T("context_item_input"), item.Name, item.Description), note
		}
	}
	for _, skill := range skills {
		if strings.EqualFold(skill.Name, target) {
			note := fmt.Sprintf(T("context_skill"), skill.Name, truncateForCLI(skill.Description, 120))
			return fmt.Sprintf(T("context_skill_input"), skill.Name, skill.Description), note
		}
	}
	return input, ""
}

func truncateForCLI(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

// 6. 管理技能
func manageSkills() {
	fmt.Println(T("skill_manage"))
	container := di.GetContainer()
	userService := container.Get("user").(*services.UserService)
	if userService == nil {
		fmt.Println("❌ 用户服务未初始化")
		return
	}

	// 暂时使用固定用户ID
	userID := defaultConsoleUserID

	fmt.Println("技能功能菜单:")
	fmt.Println("  l) 列出所有技能")
	fmt.Println("  c) 创建新技能")
	fmt.Println("  v) 查看技能详情")
	fmt.Println("  d) 删除技能")
	fmt.Println("  b) 返回主菜单")

	choice := getUserInput("请选择操作: ")

	switch strings.ToLower(choice) {
	case "l":
		skills, err := userService.GetUserSkills(userID)
		if err != nil {
			fmt.Printf("❌ 读取技能失败: %v\n", err)
			return
		}

		if len(skills) == 0 {
			fmt.Println("当前没有技能")
			return
		}

		fmt.Printf("用户 '%s' 共有 %d 个技能:\n", userID, len(skills))
		for i, skill := range skills {
			fmt.Printf("  %d) %s\n", i+1, skill.Name)
		}
	case "c":
		name := getUserInput("技能名称: ")
		description := getUserInput("技能描述: ")

		skill := models.UserSkill{
			Name:        name,
			Description: description,
			Created:     time.Now(),
			Updated:     time.Now(),
		}

		if err := userService.AddUserSkill(userID, skill); err != nil {
			fmt.Printf("❌ 添加技能失败: %v\n", err)
		} else {
			fmt.Printf("✅ 技能 '%s' 添加成功！\n", skill.Name)
		}
	case "v":
		skills, err := userService.GetUserSkills(userID)
		if err != nil {
			fmt.Printf("❌ 读取技能失败: %v\n", err)
			return
		}
		if len(skills) == 0 {
			fmt.Println("没有技能")
			return
		}
		// 简单列表选择
		for i, s := range skills {
			fmt.Printf("%d) %s\n", i+1, s.Name)
		}
		idxStr := getUserInput("输入编号: ")
		var idx int
		fmt.Sscanf(idxStr, "%d", &idx)
		if idx > 0 && idx <= len(skills) {
			s := skills[idx-1]
			printBox("技能详情", fmt.Sprintf("名称: %s\n描述: %s\nID: %s", s.Name, s.Description, s.ID))
		}
	case "d":
		skills, err := userService.GetUserSkills(userID)
		if err != nil {
			fmt.Printf("❌ 读取技能失败: %v\n", err)
			return
		}
		if len(skills) == 0 {
			fmt.Println("没有技能")
			return
		}
		for i, s := range skills {
			fmt.Printf("%d) %s\n", i+1, s.Name)
		}
		idxStr := getUserInput("输入编号删除: ")
		var idx int
		fmt.Sscanf(idxStr, "%d", &idx)
		if idx > 0 && idx <= len(skills) {
			s := skills[idx-1]
			if err := userService.DeleteUserSkill(userID, s.ID); err != nil {
				fmt.Printf("❌ 删除失败: %v\n", err)
			} else {
				fmt.Println("✅ 删除成功")
			}
		}
	case "b":
		return
	}
}

// 8. 导出故事
func exportStory() {
	fmt.Println(T("export_title"))
	container := di.GetContainer()
	exportService := container.Get("export").(*services.ExportService)
	if exportService == nil {
		fmt.Println("❌ 导出服务未初始化")
		return
	}

	sceneID := getUserInput(T("enter_scene_id"))
	if sceneID == "" {
		fmt.Println(T("scene_id_empty"))
		return
	}

	format := getUserInputWithDefault("导出格式 (json/markdown/txt/html)", "markdown")

	fmt.Println("正在导出...")
	result, err := exportService.ExportInteractionSummary(context.Background(), sceneID, format)
	if err != nil {
		fmt.Printf("❌ 导出失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 导出成功！\n文件路径: %s\n大小: %d 字节\n", result.FilePath, result.FileSize)
}
