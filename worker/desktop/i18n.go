package main

// The texts of the interface of the client.
//
// The client asks the language at the first start, the text of the interface
// follows the language of the user from then on.

var clientTexts = map[string]map[string]string{
	"tr": {
		"setup": "Kurulum", "language": "Dil", "privacy": "Gizlilik politikası",
		"terms": "Kullanım şartları", "accept_privacy": "Gizlilik politikasını okudum ve kabul ediyorum",
		"accept_terms": "Kullanım şartlarını okudum ve kabul ediyorum", "continue": "Devam",
		"connect": "Bağlan", "test": "Test", "panel": "Panele git", "quit": "Çıkış",
		"server": "Sunucu", "device": "Cihaz", "model": "Model", "speed": "Hız",
		"server_url": "Sunucu adresi", "pairing": "Eşleştirme kodu", "idle": "Beklemede",
		"connecting": "Bağlanılıyor...", "online": "Çevrimiçi", "offline": "Çevrimdışı",
		"downloading": "İndiriliyor...", "policy_title": "Gizlilik ve kullanım şartları",
		"setup_intro":       "Devam etmek için dilinizi seçin ve aşağıdaki politikaları kabul edin.",
		"setup_policy_site": "Politikaların tamamı şu adreste yayınlanır:",
		"show":              "Client'i Göster", "go_site": "Siteye git", "site_settings": "Ayarlar (Site)", "close_app": "Kapat",
		"change_language": "Dili değiştir",
		"menu_status":     "Durum", "menu_settings": "Ayarlar",
	},
	"en": {
		"setup": "Setup", "language": "Language", "privacy": "Privacy policy",
		"terms": "Terms of service", "accept_privacy": "I have read and accept the privacy policy",
		"accept_terms": "I have read and accept the terms of service", "continue": "Continue",
		"connect": "Connect", "test": "Test", "panel": "Open panel", "quit": "Quit",
		"server": "Server", "device": "Device", "model": "Model", "speed": "Speed",
		"server_url": "Server address", "pairing": "Pairing code", "idle": "Idle",
		"connecting": "Connecting...", "online": "Online", "offline": "Offline",
		"downloading": "Downloading...", "policy_title": "Privacy and terms of service",
		"setup_intro":       "Choose your language and accept the policies below to continue.",
		"setup_policy_site": "The full policies are published at:",
		"show":              "Show client", "go_site": "Go to site", "site_settings": "Settings (Site)", "close_app": "Close",
		"change_language": "Change language",
		"menu_status":     "Status", "menu_settings": "Settings",
	},
	"zh_CN": {
		"setup": "安装", "language": "语言", "privacy": "隐私政策",
		"terms": "使用条款", "accept_privacy": "我已阅读并同意隐私政策",
		"accept_terms": "我已阅读并同意使用条款", "continue": "继续",
		"connect": "连接", "test": "测试", "panel": "打开面板", "quit": "退出",
		"server": "服务器", "device": "设备", "model": "模型", "speed": "速度",
		"server_url": "服务器地址", "pairing": "配对码", "idle": "等待中",
		"connecting": "连接中...", "online": "在线", "offline": "离线",
		"downloading": "下载中...", "policy_title": "隐私政策和使用条款",
		"setup_intro":       "选择您的语言并接受以下政策以继续。",
		"setup_policy_site": "完整政策发布于：",
		"show":              "显示客户端", "go_site": "前往网站", "site_settings": "设置（网站）", "close_app": "关闭",
		"change_language": "更改语言",
		"menu_status":     "状态", "menu_settings": "设置",
	},
	"zh_TW": {
		"setup": "安裝", "language": "語言", "privacy": "隱私政策",
		"terms": "使用條款", "accept_privacy": "我已閱讀並同意隱私政策",
		"accept_terms": "我已閱讀並同意使用條款", "continue": "繼續",
		"connect": "連線", "test": "測試", "panel": "開啟面板", "quit": "離開",
		"server": "伺服器", "device": "裝置", "model": "模型", "speed": "速度",
		"server_url": "伺服器位址", "pairing": "配對碼", "idle": "等待中",
		"connecting": "連線中...", "online": "線上", "offline": "離線",
		"downloading": "下載中...", "policy_title": "隱私政策和使用條款",
		"setup_intro":       "選擇您的語言並接受以下政策以繼續。",
		"setup_policy_site": "完整政策發布於：",
		"show":              "顯示用戶端", "go_site": "前往網站", "site_settings": "設定（網站）", "close_app": "關閉",
		"change_language": "更改語言",
		"menu_status":     "狀態", "menu_settings": "設定",
	},
	"fr": {
		"setup": "Installation", "language": "Langue", "privacy": "Politique de confidentialité",
		"terms": "Conditions d'utilisation", "accept_privacy": "J'ai lu et j'accepte la politique de confidentialité",
		"accept_terms": "J'ai lu et j'accepte les conditions d'utilisation", "continue": "Continuer",
		"connect": "Se connecter", "test": "Tester", "panel": "Ouvrir le panneau", "quit": "Quitter",
		"server": "Serveur", "device": "Appareil", "model": "Modèle", "speed": "Vitesse",
		"server_url": "Adresse du serveur", "pairing": "Code d'appairage", "idle": "En attente",
		"connecting": "Connexion...", "online": "En ligne", "offline": "Hors ligne",
		"downloading": "Téléchargement...", "policy_title": "Confidentialité et conditions d'utilisation",
		"setup_intro":       "Choisissez votre langue et acceptez les politiques ci-dessous pour continuer.",
		"setup_policy_site": "Les politiques complètes sont publiées sur :",
		"show":              "Afficher le client", "go_site": "Aller sur le site", "site_settings": "Paramètres (Site)", "close_app": "Fermer",
		"change_language": "Changer de langue",
		"menu_status":     "État", "menu_settings": "Paramètres",
	},
	"ru": {
		"setup": "Установка", "language": "Язык", "privacy": "Политика конфиденциальности",
		"terms": "Условия использования", "accept_privacy": "Я прочитал и принимаю политику конфиденциальности",
		"accept_terms": "Я прочитал и принимаю условия использования", "continue": "Продолжить",
		"connect": "Подключить", "test": "Тест", "panel": "Открыть панель", "quit": "Выход",
		"server": "Сервер", "device": "Устройство", "model": "Модель", "speed": "Скорость",
		"server_url": "Адрес сервера", "pairing": "Код сопряжения", "idle": "Ожидание",
		"connecting": "Подключение...", "online": "В сети", "offline": "Не в сети",
		"downloading": "Загрузка...", "policy_title": "Конфиденциальность и условия использования",
		"setup_intro":       "Выберите язык и примите указанные ниже условия, чтобы продолжить.",
		"setup_policy_site": "Полные условия опубликованы по адресу:",
		"show":              "Показать клиент", "go_site": "Перейти на сайт", "site_settings": "Настройки (сайт)", "close_app": "Закрыть",
		"change_language": "Сменить язык",
		"menu_status":     "Состояние", "menu_settings": "Настройки",
	},
	"ja": {
		"setup": "セットアップ", "language": "言語", "privacy": "プライバシーポリシー",
		"terms": "利用規約", "accept_privacy": "プライバシーポリシーを読み、同意します",
		"accept_terms": "利用規約を読み、同意します", "continue": "続ける",
		"connect": "接続", "test": "テスト", "panel": "パネルを開く", "quit": "終了",
		"server": "サーバー", "device": "デバイス", "model": "モデル", "speed": "速度",
		"server_url": "サーバーアドレス", "pairing": "ペアリングコード", "idle": "待機中",
		"connecting": "接続中...", "online": "オンライン", "offline": "オフライン",
		"downloading": "ダウンロード中...", "policy_title": "プライバシーと利用規約",
		"setup_intro":       "言語を選択し、以下のポリシーに同意して続行してください。",
		"setup_policy_site": "完全なポリシーは次に公開されています:",
		"show":              "クライアントを表示", "go_site": "サイトへ移動", "site_settings": "設定（サイト）", "close_app": "閉じる",
		"change_language": "言語を変更",
		"menu_status":     "状態", "menu_settings": "設定",
	},
	"vi": {
		"setup": "Cài đặt", "language": "Ngôn ngữ", "privacy": "Chính sách bảo mật",
		"terms": "Điều khoản sử dụng", "accept_privacy": "Tôi đã đọc và đồng ý với chính sách bảo mật",
		"accept_terms": "Tôi đã đọc và đồng ý với điều khoản sử dụng", "continue": "Tiếp tục",
		"connect": "Kết nối", "test": "Thử", "panel": "Mở bảng điều khiển", "quit": "Thoát",
		"server": "Máy chủ", "device": "Thiết bị", "model": "Mô hình", "speed": "Tốc độ",
		"server_url": "Địa chỉ máy chủ", "pairing": "Mã ghép nối", "idle": "Đang chờ",
		"connecting": "Đang kết nối...", "online": "Trực tuyến", "offline": "Ngoại tuyến",
		"downloading": "Đang tải...", "policy_title": "Bảo mật và điều khoản sử dụng",
		"setup_intro":       "Chọn ngôn ngữ và chấp nhận các chính sách bên dưới để tiếp tục.",
		"setup_policy_site": "Toàn bộ chính sách được công bố tại:",
		"show":              "Hiện ứng dụng", "go_site": "Đến trang web", "site_settings": "Cài đặt (Trang web)", "close_app": "Đóng",
		"change_language": "Đổi ngôn ngữ",
		"menu_status":     "Trạng thái", "menu_settings": "Cài đặt",
	},
}

// clientLanguage is the language of the interface of the client.
var clientLanguage = "tr"

// clientLanguages are the languages of the installation.
var clientLanguages = []string{"tr", "en", "zh_CN", "zh_TW", "fr", "ru", "ja", "vi"}

// clientText returns the text of the interface in the language of the user.
//
// The text of English is used when the language of the user has no text.
func clientText(key string) string {
	return clientTextIn(clientLanguage, key)
}

// clientTextIn returns the text of the interface in a language.
//
// The text of English is used when the language has no text, the key itself is
// used when English has no text either.
func clientTextIn(language string, key string) string {
	if texts, ok := clientTexts[language]; ok {
		if value, ok := texts[key]; ok && value != "" {
			return value
		}
	}
	if texts, ok := clientTexts["en"]; ok {
		if value, ok := texts[key]; ok {
			return value
		}
	}
	return key
}
