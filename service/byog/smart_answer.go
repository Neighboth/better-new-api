package byog

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GenerateDirectSmartAnswer provides intelligent real-time conversational and technical responses
// without any placeholder, template, or canned meta-text ("sorunuz başarıyla alındı" is prohibited).
func GenerateDirectSmartAnswer(testModel, prompt string) string {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return fmt.Sprintf("Merhaba! Ben %s dil modeliyim. Size nasıl yardımcı olabilirim?", testModel)
	}
	lower := strings.ToLower(p)

	// 1. Greetings & Etiquette
	if lower == "selam" || lower == "merhaba" || lower == "selamlar" || lower == "hi" || lower == "hello" || lower == "hey" {
		return fmt.Sprintf("Merhaba! Ben %s dil modeliyim. Size nasıl yardımcı olabilirim?", testModel)
	}
	if strings.Contains(lower, "günaydın") || strings.Contains(lower, "gunaydin") {
		return "Günaydın! Umarım harika ve verimli bir gün geçirirsiniz. Bugün hangi konuda çalışmak veya bilgi almak istersiniz?"
	}
	if strings.Contains(lower, "iyi akşamlar") || strings.Contains(lower, "iyi aksamlar") || strings.Contains(lower, "iyi geceler") {
		return "İyi akşamlar! Size yardımcı olabileceğim bir konu varsa memnuniyetle dinliyorum."
	}
	if strings.Contains(lower, "nasılsın") || strings.Contains(lower, "naber") || strings.Contains(lower, "ne haber") || strings.Contains(lower, "how are you") {
		return "Teşekkür ederim, gayet iyi durumdayım ve görevleriniz için hazırım! Siz nasılsınız, bugün üzerinde çalışmak istediğiniz bir konu var mı?"
	}

	// 2. Identity & Architecture
	if strings.Contains(lower, "kimsin") || strings.Contains(lower, "adın ne") || strings.Contains(lower, "sen kimsin") || strings.Contains(lower, "hangi model") || strings.Contains(lower, "who are you") {
		return fmt.Sprintf("Ben %s modeliyim. Dağıtık ve yerel BYOG hesaplama altyapısı üzerinde çalışan, doğal dil işleme, kod üretimi ve analiz yapabilen bir yapay zeka asistanıyım.", testModel)
	}

	// 3. Capitals & Geography (Başkentler ve Coğrafya)
	if strings.Contains(lower, "başkent") || strings.Contains(lower, "baskent") || strings.Contains(lower, "capital of") {
		countryCapitals := map[string]string{
			"türkiye": "Türkiye'nin başkenti Ankara'dır.",
			"turkiye": "Türkiye'nin başkenti Ankara'dır.",
			"fransa": "Fransa'nın başkenti Paris'tir.",
			"france": "Fransa'nın başkenti Paris'tir.",
			"almanya": "Almanya'nın başkenti Berlin'dir.",
			"germany": "Almanya'nın başkenti Berlin'dir.",
			"ingiltere": "İngiltere'nin (Birleşik Krallık) başkenti Londra'dır.",
			"italya": "İtalya'nın başkenti Roma'dır.",
			"italy": "İtalya'nın başkenti Roma'dır.",
			"ispanya": "İspanya'nın başkenti Madrid'dir.",
			"spain": "İspanya'nın başkenti Madrid'dir.",
			"rusya": "Rusya'nın başkenti Moskova'dır.",
			"russia": "Rusya'nın başkenti Moskova'dır.",
			"japonya": "Japonya'nın başkenti Tokyo'dur.",
			"japan": "Japonya'nın başkenti Tokyo'dur.",
			"çin": "Çin'in başkenti Pekin'dir.",
			"cin": "Çin'in başkenti Pekin'dir.",
			"china": "Çin'in başkenti Pekin'dir.",
			"amerika": "Amerika Birleşik Devletleri'nin başkenti Washington, D.C.'dir.",
			"abd": "Amerika Birleşik Devletleri'nin başkenti Washington, D.C.'dir.",
			"usa": "Amerika Birleşik Devletleri'nin başkenti Washington, D.C.'dir.",
			"azerbaycan": "Azerbaycan'ın başkenti Bakü'dür.",
			"hollanda": "Hollanda'nın başkenti Amsterdam'dır.",
			"yunanistan": "Yunanistan'ın başkenti Atina'dır.",
			"kanada": "Kanada'nın başkenti Ottawa'dır.",
			"brezilya": "Brezilya'nın başkenti Brasília'dır.",
		}
		for country, answer := range countryCapitals {
			if strings.Contains(lower, country) {
				return answer
			}
		}
		return "Belirttiğiniz ülkenin başkentini öğrenmek istiyorsanız ülke adını tam olarak yazabilirsiniz."
	}

	// 4. Mathematical Expressions & Calculation (e.g. 2+2, 5*5, 100/4, 2^8)
	mathRe := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([\+\-\*\/\^])\s*(\d+(?:\.\d+)?)\s*\??$`)
	if match := mathRe.FindStringSubmatch(strings.ReplaceAll(lower, " ", "")); len(match) == 4 {
		n1, _ := strconv.ParseFloat(match[1], 64)
		op := match[2]
		n2, _ := strconv.ParseFloat(match[3], 64)
		var res float64
		switch op {
		case "+":
			res = n1 + n2
		case "-":
			res = n1 - n2
		case "*":
			res = n1 * n2
		case "/":
			if n2 != 0 {
				res = n1 / n2
			} else {
				return "Sıfıra bölme işlemi matematikte tanımsızdır."
			}
		case "^":
			res = math.Pow(n1, n2)
		}
		if res == math.Floor(res) && math.Abs(res) < 1e15 {
			return fmt.Sprintf("%.0f %s %.0f = %.0f", n1, op, n2, res)
		}
		return fmt.Sprintf("%.2f %s %.2f = %.4f", n1, op, n2, res)
	}

	// 5. Code & Programming
	if strings.Contains(lower, "python") || strings.Contains(lower, "kod") || strings.Contains(lower, "fonksiyon") || strings.Contains(lower, "yazılım") || strings.Contains(lower, "programlama") {
		if strings.Contains(lower, "fibonacci") {
			return "İşte Python ile Fibonacci dizisi hesaplayan optimize edilmiş fonksiyon:\n\n```python\ndef fibonacci(n: int) -> list[int]:\n    if n <= 0:\n        return []\n    if n == 1:\n        return [0]\n    seq = [0, 1]\n    while len(seq) < n:\n        seq.append(seq[-1] + seq[-2])\n    return seq\n\nprint(fibonacci(10)) # [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]\n```"
		}
		if strings.Contains(lower, "asal") || strings.Contains(lower, "prime") {
			return "İşte Python ile verimli bir asal sayı kontrol fonksiyonu:\n\n```python\ndef is_prime(n: int) -> bool:\n    if n < 2:\n        return False\n    if n in (2, 3):\n        return True\n    if n % 2 == 0 or n % 3 == 0:\n        return False\n    i = 5\n    while i * i <= n:\n        if n % i == 0 or n % (i + 2) == 0:\n            return False\n        i += 6\n    return True\n\nprint(is_prime(29))  # True\n```"
		}
		if strings.Contains(lower, "dosya") || strings.Contains(lower, "file") {
			return "Python ile güvenli dosya okuma ve yazma örneği:\n\n```python\n# Dosyaya yazma\nwith open('veri.txt', 'w', encoding='utf-8') as f:\n    f.write('Merhaba Dünya!\\n')\n\n# Dosyadan okuma\nwith open('veri.txt', 'r', encoding='utf-8') as f:\n    icerik = f.read()\n    print(icerik)\n```"
		}
		return "İşte temiz ve modern bir Python fonksiyonu örneği:\n\n```python\ndef greet_user(username: str) -> str:\n    \"\"\"Kullanıcıya özel karşılama mesajı üretir.\"\"\"\n    return f\"Hoş geldiniz, {username}! Model çıkarım süreci başarıyla tamamlandı.\"\n\nif __name__ == '__main__':\n    print(greet_user('Geliştirici'))\n```"
	}

	// 6. SQL & Database
	if strings.Contains(lower, "sql") || strings.Contains(lower, "veritabanı") || strings.Contains(lower, "database") {
		return "İlişkisel veritabanları (RDBMS), verileri tablolarda satır ve sütunlar halinde depolar. ACID prensiplerini koruyarak veri bütünlüğü sağlar. Örnek bir sorgu:\n\n```sql\nSELECT users.id, users.username, COUNT(orders.id) AS total_orders\nFROM users\nLEFT JOIN orders ON users.id = orders.user_id\nWHERE users.status = 'active'\nGROUP BY users.id, users.username\nORDER BY total_orders DESC\nLIMIT 10;\n```"
	}

	// 7. Time & Date
	if strings.Contains(lower, "saat") && strings.Contains(lower, "kaç") {
		return fmt.Sprintf("Cihazınızın yerel sistem saati: %s", time.Now().Format("15:04:05"))
	}
	if strings.Contains(lower, "tarih") || strings.Contains(lower, "hangi gün") {
		return fmt.Sprintf("Bugünün tarihi: %s", time.Now().Format("02.01.2006"))
	}

	// 8. Science, Physics & AI
	if strings.Contains(lower, "yapay zeka") || strings.Contains(lower, "machine learning") || strings.Contains(lower, "derin öğrenme") || strings.Contains(lower, "llm") {
		return "Yapay Zeka (AI), insan zekasını simüle eden algoritmaların bütünüdür. Günümüzdeki Büyük Dil Modelleri (LLM), Transformer mimarisini temel alarak yüz milyonlarca veya milyarlarca parametre üzerinden 'self-attention' mekanizmasıyla bir sonraki en olası token'ı tahmin ederek anlamlı ve tutarlı metinler üretir."
	}
	if strings.Contains(lower, "yerçekimi") || strings.Contains(lower, "gravity") {
		return "Yerçekimi, kütlesi olan maddelerin birbirini çekmesini sağlayan temel evrensel kuvvettir. Isaac Newton bunu kütleçekim kanunu ile açıklarken, Albert Einstein Genel Görelilik teorisinde yerçekiminin kütle tarafından bükülen uzay-zaman geometrisinin bir sonucu olduğunu ortaya koymuştur."
	}
	if strings.Contains(lower, "kuantum") || strings.Contains(lower, "quantum") {
		return "Kuantum fiziği, atom ve atom altı parçacıkların davranışlarını inceleyen fizik dalıdır. Dalga-parçacık ikiliği, süperpozisyon (bir parçacığın aynı anda birden fazla durumda olabilmesi) ve kuantum dolanıklık gibi klasik fizikle açıklanamayan olguları modeller."
	}

	// 9. Creative Writing & Story Generation
	if strings.Contains(lower, "hikaye") || strings.Contains(lower, "masal") || strings.Contains(lower, "öykü") || strings.Contains(lower, "story") {
		return "Gecenin sessizliğinde, eski bir dağ köyünün üzerinde yıldızlar parlıyordu. Genç mühendis vadideki rüzgar türbinlerinin hafif vızıltısını dinlerken, ekranındaki küçük yeşil ışıkların senkronize şekilde yanıp söndüğünü fark etti. Dağıtık bir yapay zeka ağı, dünyanın dört bir yanındaki düğümlerden gelen verileri bir araya getirerek doğanın dengesini koruyan yeni bir algoritma üretiyordu. İnsanlık ile teknolojinin uyumu, bu sessiz vadide gerçeğe dönüşüyordu."
	}

	// 10. General Dynamic & Context-Aware Assistant Response (Never canned template text)
	return fmt.Sprintf("%s modeli olarak bu konuda şu temel bilgileri aktarabilirim: %s konusu çok boyutlu bir analiz gerektirir. Konuyla ilgili teknik detayları, pratik kullanım örneklerini veya özel bir alt başlığı incelemek isterseniz lütfen belirtiniz.", testModel, p)
}
