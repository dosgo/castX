var zh_lang={
    auth_title:"安全验证",
    auth_subtitle:'请输入访问密码以继续操作',
    placeholder:'输入访问密码',
    login:'验证身份',
    loginErrMsg:"密码错误或者系统时间不对!",
    pair:'认证',
    connect:"连接",
    pair_placeholder:'请输入6位认证码',
    connect_port_placeholder:'连接端口',
    pair_port_placeholder:'认证端口',
};

var en_lang={
    auth_title:"Security Verification",
    auth_subtitle:'Please enter your access code to continue',
    placeholder:'Enter access code',
    login:'login',
    loginErrMsg:"The password is incorrect or the system time is incorrect!",
    pair:'pair',
    connect:"connect",
    pair_port_placeholder:'pair port',
    connect_port_placeholder:'connect port',
    pair_placeholder:'Please enter the 6-digit verification code',
}

function getLang(label){
    let lang= navigator.language
    if(lang.startsWith('zh')){
        return label?zh_lang[label]:zh_lang;
    }else{
        return label?en_lang[label]:en_lang;
    }
}