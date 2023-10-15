uid = CS.UnityEngine.GameObject.Find("/BetaWatermarkCanvas(Clone)/Panel/TxtUID"):GetComponent("Text")
uid.text = "<color=#00dbe5>" .. uid.text:gsub("UID:", "<color=#c119b1>删档同步测试服:</color>") .. "</color>"
