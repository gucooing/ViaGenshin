-- 日志打印
function logDebug(msg)
    -- CS.MoleMole.ActorUtils.ShowMessage(msg)
end
function logInfo(msg)
    CS.MoleMole.ActorUtils.ShowMessage(msg)
end

-- 异常回调
function onError(error)
    logDebug(error)
end

-- 创建功能按钮
function createFuncBtn(name, text, func, pos)
    local obj = CS.UnityEngine.GameObject(name)
    -- 空白背景图
    obj:AddComponent(typeof(CS.UnityEngine.UI.Image))
    -- 按钮
    local btn = obj:AddComponent(typeof(CS.UnityEngine.UI.Button))
    btn.onClick:AddListener(func)
    -- 复制一个uid框拿来用
    local txt = CS.UnityEngine.GameObject.Instantiate(CS.UnityEngine.GameObject.Find("/BetaWatermarkCanvas(Clone)/Panel/TxtUID"))
    txt:GetComponent("Text").text = "<color=#66ccff><size=18>" .. text .. "</size></color>"
    txt.transform:SetParent(obj.transform)
    txt.transform.localScale = CS.UnityEngine.Vector3(1.5, 3, 0)
    txt.transform.localPosition = CS.UnityEngine.Vector3(50, -30, 0)
    -- 画布位置
    local canvas = CS.UnityEngine.GameObject.Find("/Canvas")
    obj.transform:SetParent(canvas.transform)
    obj.transform.localScale = CS.UnityEngine.Vector3(1, 0.5, 0)
    obj.transform.localPosition = pos
    obj:SetActive(false)
    return obj
end

gmMenuFlag = false
accSwitchValue = 0.5
fpsSwitchValue = 10
fogSwitchFlag = false
testModeFlag = false

-- 初始化GM菜单
function initGmMenu()
    logDebug("initGmMenu")
    -- 开启live版本客户端的GM按钮
    local btnGm = CS.UnityEngine.GameObject.Find("/Canvas/Pages/InLevelMainPage/GrpMainPage/GrpMainBtn/GrpMainToggle/GrpTopPanel/BtnGm")
    btnGm:SetActive(true)
    -- 切换加速
    local accSwitch = createFuncBtn("accSwitch",
            "切换加速",
            function()
                CS.UnityEngine.Time.timeScale = accSwitchValue
                logInfo("当前加速倍率: " .. accSwitchValue)
                accSwitchValue = accSwitchValue + 0.5
                if accSwitchValue >= 10.0 then
                    accSwitchValue = 0.5
                end
            end,
            CS.UnityEngine.Vector3(-400, 200, 0))
    -- 切换帧率
    local fpsSwitch = createFuncBtn("fpsSwitch",
            "切换帧率",
            function()
                CS.UnityEngine.Application.targetFrameRate = fpsSwitchValue
                logInfo("当前帧率: " .. fpsSwitchValue)
                fpsSwitchValue = fpsSwitchValue + 10
                if fpsSwitchValue >= 200 then
                    fpsSwitchValue = 10
                end
            end,
            CS.UnityEngine.Vector3(-200, 200, 0))
    -- 迷雾开关
    local fogSwitch = createFuncBtn("fogSwitch",
            "迷雾开关",
            function()
                if fogSwitchFlag then
                    CS.UnityEngine.RenderSettings.fog = true
                    fogSwitchFlag = false
                else
                    CS.UnityEngine.RenderSettings.fog = false
                    fogSwitchFlag = true
                end
                logInfo("迷雾开启: " .. tostring(CS.UnityEngine.RenderSettings.fog))
            end,
            CS.UnityEngine.Vector3(0, 200, 0))
    -- 测试模式
    local testMode = createFuncBtn("testMode",
            "测试模式",
            function()
                local mainCamera = CS.UnityEngine.GameObject.FindGameObjectWithTag("MainCamera")
                local camera = mainCamera:GetComponent(typeof(CS.UnityEngine.Camera))
                if testModeFlag then
                    camera.renderingPath = CS.UnityEngine.RenderingPath.DeferredLighting
                    camera.allowHDR = true
                    testModeFlag = false
                else
                    camera.renderingPath = CS.UnityEngine.RenderingPath.Forward
                    camera.allowHDR = false
                    testModeFlag = true
                end
            end,
            CS.UnityEngine.Vector3(200, 200, 0))
    -- 注册GM按钮回调
    local button = btnGm:GetComponent(typeof(CS.UnityEngine.UI.Button))
    local function showGmMenu()
        accSwitch:SetActive(true)
        fogSwitch:SetActive(true)
        fpsSwitch:SetActive(true)
        testMode:SetActive(true)
    end
    local function hideGmMenu()
        accSwitch:SetActive(false)
        fogSwitch:SetActive(false)
        fpsSwitch:SetActive(false)
        testMode:SetActive(false)
    end
    button.onClick:AddListener(
            function()
                showGmMenu()
                if gmMenuFlag then
                    hideGmMenu()
                    gmMenuFlag = false
                else
                    showGmMenu()
                    gmMenuFlag = true
                end
            end)
    logDebug("initGmMenu ok")
end
xpcall(initGmMenu, onError)
