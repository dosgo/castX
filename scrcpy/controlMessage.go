package scrcpy

//https://github.com/Genymobile/scrcpy/server/src/main/java/com/genymobile/scrcpy/control/ControlMessage.java

var TYPE_INJECT_KEYCODE byte = 0          //输入入键盘
var TYPE_INJECT_TEXT byte = 1             //输入文本
var TYPE_INJECT_TOUCH_EVENT byte = 2      //输入触摸事件
var TYPE_INJECT_SCROLL_EVENT byte = 3     //输入滚动事件
var TYPE_BACK_OR_SCREEN_ON = 4            //返回或者屏幕开
var TYPE_EXPAND_NOTIFICATION_PANEL = 5    //展开通知面板
var TYPE_EXPAND_SETTINGS_PANEL = 6        //展开设置面板
var TYPE_COLLAPSE_PANELS = 7              //收起面板
var TYPE_GET_CLIPBOARD = 8                //获取剪贴板
var TYPE_SET_CLIPBOARD = 9                //设置剪贴板
var TYPE_SET_DISPLAY_POWER byte = 10      //关闭屏幕
var TYPE_ROTATE_DEVICE = 11               //旋转屏幕
var TYPE_UHID_CREATE = 12                 //创建uhid
var TYPE_UHID_INPUT = 13                  //uhid输入
var TYPE_UHID_DESTROY = 14                //销毁uhid
var TYPE_OPEN_HARD_KEYBOARD_SETTINGS = 15 //打开硬件键盘设置
var TYPE_START_APP = 16                   //启动应用
var TYPE_RESET_VIDEO = 17

//android keycode ev
var ACTION_DOWN byte = 0
var ACTION_UP byte = 1
var ACTION_MOVE byte = 2

//android mouse event

var BUTTON_PRIMARY uint32 = 1 << 0

/**
 * Button constant: Secondary button (right mouse button).
 *
 * @see #getButtonState
 */
var BUTTON_SECONDARY uint32 = 1 << 1

/**
 * Button constant: Tertiary button (middle mouse button).
 *
 * @see #getButtonState
 */
var BUTTON_TERTIARY uint32 = 1 << 2
