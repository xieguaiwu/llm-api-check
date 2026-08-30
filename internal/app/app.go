// Package app 刷新编排，对应 Android 版 AppViewModel.kt 的数据层职责
// （CLI 无持续 UI，单次执行：刷新 → 渲染 → 退出，故不保留旧数据）。
package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xieguiawu/llm-api-check/internal/config"
	"github.com/xieguiawu/llm-api-check/internal/models"
	"github.com/xieguiawu/llm-api-check/internal/repo"
)

// DeepSeekResult 单个 DeepSeek 账号的刷新结果（对应 DeepSeekUi）。
// Error 合并 balance/cost 两者错误（"\n" 连接，无错误则空）。
type DeepSeekResult struct {
	Account models.DeepSeekAccount  `json:"account"`
	Balance *models.DeepSeekBalance `json:"balance,omitempty"`
	Cost    *models.DeepSeekCost    `json:"cost,omitempty"`
	Error   string                  `json:"error,omitempty"`
}

// AccountResult 单个 OpenCode 账号的刷新结果（对应 AccountUi）。
type AccountResult struct {
	Account    models.Account     `json:"account"`
	GoUsage    *models.GoUsage    `json:"go_usage,omitempty"`
	ZenBilling *models.ZenBilling `json:"zen_billing,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// QwenResult 单个 Qwen 账号的刷新结果（对应 QwenUi）。
// Plan 走 API Key（模型清单）；Usage 走 Bailian CLI 或控制台 Cookie（配额窗口）；
// Stats 走 Bailian CLI（用量分析，--stats 时才拉）。
type QwenResult struct {
	Account models.QwenAccount  `json:"account"`
	Plan    *models.QwenPlan    `json:"plan,omitempty"`
	Usage   *models.QwenUsage   `json:"usage,omitempty"`
	Stats   *models.QwenSummary `json:"stats,omitempty"`
	Error   string              `json:"error,omitempty"`
	// CLIEnabled 本机是否探测到 Bailian CLI（渲染层区分“暂无数据”与“未配置凭据”）
	CLIEnabled bool `json:"-"`
	// CLILoginCmd / CLIInstallCmd 可直接粘贴的登录与安装命令（含真实路径，
	// 不用裸 bailian——默认独立 prefix 安装不在 PATH）
	CLILoginCmd   string `json:"-"`
	CLIInstallCmd string `json:"-"`
}

// GalaxyResult 单个智星云账号的刷新结果（对应 GalaxyUi）。
// Balance 必需；Status/Instances/Cost 任一失败只影响该段（错误合并进 Error）。
type GalaxyResult struct {
	Account   models.GalaxyAccount      `json:"account"`
	Balance   *models.GalaxyBalance     `json:"balance,omitempty"`
	Status    *models.GalaxyStatusCount `json:"status,omitempty"`
	Instances []models.GalaxyInstance   `json:"instances,omitempty"`
	Cost      *models.GalaxyCost        `json:"cost,omitempty"`
	Error     string                    `json:"error,omitempty"`
}

// HourlyCost 运行中实例的合计时价（元/时）——余额还能撑多久算得出来
func (r GalaxyResult) HourlyCost() float64 {
	var sum float64
	for _, in := range r.Instances {
		if in.Status == 1 || in.Status == 4 || in.Status == 5 {
			sum += in.TotalCost
		}
	}
	return sum
}

// Result 全量刷新结果
type Result struct {
	DeepSeek    []DeepSeekResult
	Accounts    []AccountResult
	Qwen        []QwenResult
	Galaxy      []GalaxyResult
	LastUpdated time.Time
}

// Repos 仓库注入点（测试可替换为 httptest 服务）
type Repos struct {
	DeepSeek *repo.DeepSeekRepo
	OpenCode *repo.OpenCodeRepo
	Qwen     *repo.QwenRepo
	Galaxy   *repo.GalaxyRepo
}

// GalaxyInstanceLimit 单次刷新展示的活跃实例上限（防止大账号拉穿）
const GalaxyInstanceLimit = 20

// App 刷新编排器（对应 AppViewModel）
type App struct {
	Repos *Repos
	Cfg   *config.Config

	mu         sync.Mutex
	refreshing bool
}

// New 默认构造：真实端点 + 15s 超时 client
func New(cfg *config.Config) *App {
	return &App{
		Repos: &Repos{
			DeepSeek: repo.NewDeepSeekRepo(),
			OpenCode: repo.NewOpenCodeRepo(),
			Qwen:     repo.NewQwenRepo(),
			Galaxy:   repo.NewGalaxyRepo(),
		},
		Cfg: cfg,
	}
}

// NewWithRepos 测试注入仓库
func NewWithRepos(cfg *config.Config, repos *Repos) *App {
	return &App{Repos: repos, Cfg: cfg}
}

// RefreshAll 刷新全部（DeepSeek 全账号 + OpenCode 全账号，并行）。
// 重入保护：刷新进行中再次调用直接忽略（对应 Android 版 refreshing 判断）。
// 以配置为准重建账号列表；成功后写入 Config.LastUpdate["all"]。
func (a *App) RefreshAll() (Result, error) {
	a.mu.Lock()
	if a.refreshing {
		a.mu.Unlock()
		return Result{}, nil
	}
	a.refreshing = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.refreshing = false
		a.mu.Unlock()
	}()

	dsAccounts := append([]models.DeepSeekAccount(nil), a.Cfg.DeepSeekAccounts...)
	accounts := append([]models.Account(nil), a.Cfg.Accounts...)
	qwenAccounts := append([]models.QwenAccount(nil), a.Cfg.QwenAccounts...)
	galaxyAccounts := append([]models.GalaxyAccount(nil), a.Cfg.GalaxyAccounts...)
	dsRes := make([]DeepSeekResult, len(dsAccounts))
	accRes := make([]AccountResult, len(accounts))
	qwenRes := make([]QwenResult, len(qwenAccounts))
	galaxyRes := make([]GalaxyResult, len(galaxyAccounts))
	var wg sync.WaitGroup
	for i, acc := range dsAccounts {
		wg.Add(1)
		go func(i int, acc models.DeepSeekAccount) {
			defer wg.Done()
			dsRes[i] = a.refreshDeepSeek(acc)
		}(i, acc)
	}
	for i, acc := range accounts {
		wg.Add(1)
		go func(i int, acc models.Account) {
			defer wg.Done()
			accRes[i] = a.refreshAccount(acc)
		}(i, acc)
	}
	for i, acc := range qwenAccounts {
		wg.Add(1)
		go func(i int, acc models.QwenAccount) {
			defer wg.Done()
			qwenRes[i] = a.refreshQwen(acc)
		}(i, acc)
	}
	for i, acc := range galaxyAccounts {
		wg.Add(1)
		go func(i int, acc models.GalaxyAccount) {
			defer wg.Done()
			galaxyRes[i] = a.refreshGalaxy(acc, GalaxyInstanceLimit)
		}(i, acc)
	}
	wg.Wait()
	now := time.Now()
	a.Cfg.SetLastUpdate("all", now.UnixMilli())
	return Result{DeepSeek: dsRes, Accounts: accRes, Qwen: qwenRes, Galaxy: galaxyRes, LastUpdated: now}, nil
}

// RefreshDeepSeek 按 id 刷新单个 DeepSeek 账号（对应 refreshDeepSeekNow）
func (a *App) RefreshDeepSeek(id string) (DeepSeekResult, error) {
	var acc models.DeepSeekAccount
	found := false
	for _, x := range a.Cfg.DeepSeekAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return DeepSeekResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshDeepSeek(acc), nil
}

// refreshDeepSeek 刷余额 + 消费（配置了 token 才拉消费），错误合并。
// 对应 AppViewModel.refreshDeepSeekNow 的 listOfNotNull(...).joinToString("\n") 逻辑。
func (a *App) refreshDeepSeek(acc models.DeepSeekAccount) DeepSeekResult {
	res := DeepSeekResult{Account: acc}
	bal, balErr := a.Repos.DeepSeek.Balance(acc.ApiKey)
	if balErr == nil {
		res.Balance = &bal
	}
	if acc.HasToken() {
		cost, costErr := a.Repos.DeepSeek.Cost(acc.PlatformToken)
		if costErr == nil {
			res.Cost = &cost
		}
		res.Error = joinErrors(balErr, costErr)
	} else {
		res.Error = errMsg(balErr)
	}
	return res
}

// RefreshAccount 按 id 刷新单个 OpenCode 账号（对应 refreshAccountNow）
func (a *App) RefreshAccount(id string) (AccountResult, error) {
	var acc models.Account
	found := false
	for _, x := range a.Cfg.Accounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return AccountResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshAccount(acc), nil
}

// refreshAccount 刷 Go usage + Zen billing（配置了 workspace/cookie 才拉 Zen）。
func (a *App) refreshAccount(acc models.Account) AccountResult {
	res := AccountResult{Account: acc}
	goU, goErr := a.Repos.OpenCode.GoUsage(acc)
	if goErr == nil {
		res.GoUsage = &goU
	}
	if acc.HasZen() {
		zen, zenErr := a.Repos.OpenCode.ZenBilling(acc)
		if zenErr == nil {
			res.ZenBilling = &zen
		}
		res.Error = joinErrors(goErr, zenErr)
	} else {
		res.Error = errMsg(goErr)
	}
	return res
}

// RefreshQwen 按 id 刷新单个 Qwen 账号（对应 refreshQwenNow）
func (a *App) RefreshQwen(id string) (QwenResult, error) {
	var acc models.QwenAccount
	found := false
	for _, x := range a.Cfg.QwenAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return QwenResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshQwen(acc), nil
}

// RefreshQwenStats 拉用量分析（token 统计 + 免费额度，仅 Bailian CLI 通道）。
func (a *App) RefreshQwenStats(id string) (QwenResult, error) {
	var acc models.QwenAccount
	found := false
	for _, x := range a.Cfg.QwenAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return QwenResult{}, errors.New("账号不存在或已被删除")
	}
	if !a.Repos.Qwen.CLIEnabled() {
		return QwenResult{}, fmt.Errorf("用量分析需要 Bailian CLI：本机未探测到。安装：%s；登录：%s",
			a.Repos.Qwen.CLIInstallCmd(), a.Repos.Qwen.CLILoginCmd())
	}
	s, err := a.Repos.Qwen.CLI.Summary(acc)
	if err != nil {
		return QwenResult{}, err
	}
	return QwenResult{Account: acc, Stats: &s}, nil
}

// RefreshQwen 刷模型清单（API Key）+ 配额窗口（Bailian CLI 或 Cookie 任一可用时拉取），错误合并。
func (a *App) refreshQwen(acc models.QwenAccount) QwenResult {
	res := QwenResult{Account: acc,
		CLIEnabled:    a.Repos.Qwen.CLIEnabled(),
		CLILoginCmd:   a.Repos.Qwen.CLILoginCmd(),
		CLIInstallCmd: a.Repos.Qwen.CLIInstallCmd(),
	}
	plan, planErr := a.Repos.Qwen.Plan(acc)
	if planErr == nil {
		res.Plan = &plan
	}
	if acc.HasCookie() || a.Repos.Qwen.CLIEnabled() {
		usage, usageErr := a.Repos.Qwen.Usage(acc)
		if usageErr == nil {
			res.Usage = &usage
		}
		res.Error = joinErrors(planErr, usageErr)
	} else {
		res.Error = errMsg(planErr)
	}
	return res
}

// RefreshGalaxy 按 id 刷新单个智星云账号（对应 refreshGalaxyNow）。
// limit ≤0 表示实例列表不限量（受仓库翻页上限约束）。
func (a *App) RefreshGalaxy(id string, limit int) (GalaxyResult, error) {
	var acc models.GalaxyAccount
	found := false
	for _, x := range a.Cfg.GalaxyAccounts {
		if x.ID == id {
			acc = x
			found = true
			break
		}
	}
	if !found {
		return GalaxyResult{}, errors.New("账号不存在或已被删除")
	}
	return a.refreshGalaxy(acc, limit), nil
}

// refreshGalaxy 并发拉四类数据（余额/统计/实例/消耗）。
// 任一路失败不中断其余路：余额拉到就显示余额（同 DeepSeek balance/cost 的处理），
// 全部失败时上层据「无数据 + 有错误」判 exit 1。
func (a *App) refreshGalaxy(acc models.GalaxyAccount, limit int) GalaxyResult {
	res := GalaxyResult{Account: acc}
	if a.Repos == nil || a.Repos.Galaxy == nil {
		res.Error = "智星云仓库未初始化"
		return res
	}
	var (
		wg      sync.WaitGroup
		bal     models.GalaxyBalance
		balErr  error
		cnt     models.GalaxyStatusCount
		cntErr  error
		insts   []models.GalaxyInstance
		insErr  error
		cost    models.GalaxyCost
		costErr error
	)
	fetch := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	fetch(func() { bal, balErr = a.Repos.Galaxy.Balance(acc) })
	fetch(func() { cnt, cntErr = a.Repos.Galaxy.StatusCount(acc) })
	fetch(func() { insts, insErr = a.Repos.Galaxy.Instances(acc, repo.GalaxyStatusDefault, limit) })
	fetch(func() { cost, costErr = a.Repos.Galaxy.Cost(acc) })
	wg.Wait()

	if balErr == nil {
		res.Balance = &bal
	}
	if cntErr == nil {
		res.Status = &cnt
	}
	if insErr == nil {
		res.Instances = insts
	}
	if costErr == nil {
		res.Cost = &cost
	}
	res.Error = joinErrors(balErr, cntErr, insErr, costErr)
	return res
}

// LastUpdated 从配置读取最近一次全量刷新时间（对应 SecureSettings.lastUpdate("all")）
func (a *App) LastUpdated() time.Time {
	if a.Cfg == nil {
		return time.Time{}
	}
	return a.Cfg.LastUpdateAt("all")
}

// joinErrors 合并错误消息（"\n" 连接，全部 nil → 空串）
func joinErrors(errs ...error) string {
	var msgs []string
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, e.Error())
		}
	}
	return strings.Join(msgs, "\n")
}

func errMsg(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
