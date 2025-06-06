 
    function calculateSize() {
        const scale =  1;
        // 保持原有屏幕尺寸获取方式，但增加全屏判断
        let containerWidth = window.screen.availWidth * scale;
        let containerHeight = window.screen.availHeight * scale;
        let _targetWidth=0;
        let _targetHeight=0;

        const videoAspect = nativeWidth / nativeHeight;
        const containerAspect = containerWidth / containerHeight;

        if (containerAspect > videoAspect) {
            // 容器更宽：高度撑满，宽度按比例
            _targetHeight = containerHeight;
            _targetWidth = _targetHeight * videoAspect;
        } else {
            // 容器更高：宽度撑满，高度按比例
            _targetWidth = containerWidth;
            _targetHeight = containerWidth / videoAspect;
        }
        targetWidth=_targetWidth;
        targetHeight=_targetHeight;
    }
   

var videoVm = Vue.createApp({
    data() { 
        return { 
            isPlaying: false,
            remoteVideo: null, // 用于保存 video 引用
            isMuted:false,
            isFullScreen: false,
            isAndroid:false,
            isAuth:false,
            useAdb:false,
            password:'',
            displayPower:true, // 显示开关状态
            errorMessage:'',
            lang:{},
        }
    
    },
  mounted() {
    // 获取 video 元素引用
    this.remoteVideo = document.getElementById('remoteVideo');
    this.isMuted = this.remoteVideo.muted;
    // 添加事件监听
    this.remoteVideo.addEventListener('play', () => this.isPlaying = true);
    this.remoteVideo.addEventListener('pause', () => this.isPlaying = false);
    this.addFullscreenListener();
    this.lang=getLang();
  },
  methods: {
     togglePlay() {
            if (this.remoteVideo.paused) {
                this.remoteVideo.play()
                    .then(() => {
                     
                    })
                    .catch(error => {
                        console.log('播放被阻止:', error);
                    });
            } else {
               this.remoteVideo.pause();
              
            }
    },
    togglePlayCanvas() {
        if (this.remoteVideo.paused) {
            this.remoteVideo.play()
                .then(() => {
                    initWebGL(this.remoteVideo);
                    canvasSizev1();
                })
                .catch(error => {
                    console.log('播放被阻止:', error);
                });
        } else {
            stopRender();
            this.remoteVideo.pause();
        }
    },
     toggleMute() {
        this.isMuted = !this.isMuted;
        remoteVideo.muted = this.isMuted;
    },
    toggleMiniPlay() {
        remoteVideo.requestPictureInPicture();
    },
    
    addFullscreenListener() {
      const events = [
        'fullscreenchange',
        'webkitfullscreenchange',
        'mozfullscreenchange'
      ]
      
      events.forEach(event => {
        document.addEventListener(event, this.handleFullscreenChange)
      })
    },
     toggleFullScreen() {
      const videoBox = document.getElementById('videoBox')
      
      try {
        if (!document.fullscreenElement) {
           videoBox.requestFullscreen()
          if (checkDevice() === 'mobile') {
             screen.orientation.lock("landscape")
          }
       
        } else {
           document.exitFullscreen()
        }
      } catch (error) {
        console.error('全屏操作失败:', error)
      }
    },
    handleFullscreenChange() {
        this.isFullScreen = !!(
            document.fullscreenElement ||
            document.webkitFullscreenElement ||
            document.mozFullScreenElement
        )
    },
    checkPassword(){
        localStorage.setItem('password',this.password)
        if (typeof login === 'function') {
            login();
        }
    },
    sendDisplayPower() {
      this.displayPower=!this.displayPower;
      var args= JSON.stringify({"type":'displayPower',"action":this.displayPower?1:0})
      ws.send(JSON.stringify({
          type: 'control',
          data: args
      }));
    },
  }

}).mount('#videoBox');