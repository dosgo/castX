var appvm = Vue.createApp({

    data() { 
        return { 
            showMenu: false,
            isConnected: true,
            config:JSON.parse(localStorage.getItem('config')) || {"selectedType":"wifi"},
            lang:getLang(),//语言
        }
      
      },
      mounted() {
        this.lang=getLang();
      },
      methods: {
        toggleMenu() {
          this.showMenu = !this.showMenu
        },
        selectType(type) {
          this.config.selectedType = type
        },
       
        connectUsb(device) {
          console.log('Connecting to:', device.name)
          // 这里添加USB连接逻辑
          this.isConnected = true
          this.showMenu = false
        },
        connectDevice(adbType){
          this.config.adbType=adbType;//"connect";
          this.config.max_size=screen.width>screen.height?screen.width:screen.height;
          var args=  JSON.stringify(this.config)
            ws.send(JSON.stringify({
                type: 'connectAdb',
                data: args
            }));
            if (adbType=="pair"){
              this.config.authPort='';
              this.config.authCode='';
            }
        }
      },
      watch: {
        // 监听整个对象的深层次变化
        config: {
          handler(newVal, oldVal) {
            localStorage.setItem('config', JSON.stringify(this.config));
            console.log('config changed:', newVal, oldVal);
          },
          deep: true // 关键：开启深度监听
        }
      }
    }).mount('#app');