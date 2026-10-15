1. **Homepage Content Tema Uyumu ve Anlık Güncelleme**
   - `web/src/features/home/index.tsx` dosyasında `iframe`'in `src` özelliğine tema parametresi (`?theme=${resolvedTheme}`) eklenecek.

2. **Tema Değişiminde Logo Gecikmesi**
   - `web/src/hooks/use-system-config.ts` dosyasında bulunan logoları preload eden fonksiyonda; sayfa ilk yüklendiğinde hem aydınlık (`config.logo`) hem de karanlık (`config.logoDark`) temanın logoları önceden belleğe alınacak şekilde (`preloadImage` kullanılarak) güncellenecek.

3. **Homepage Content İframe İnternet Erişimi**
   - `web/src/features/home/index.tsx` içerisindeki `iframe` elementinde yer alan `sandbox` niteliklerine `allow-same-origin` parametresi eklenecek.

4. **Sistem Bilgilerindeki Kullanıcı Sözleşmesi ve Gizlilik Politikası (Kayıt Sorunu)**
   - `web/src/features/system-settings/general/system-info-section.tsx` dosyasında form maplenirken kullanılan verilerin `defaultValues` içerisinden okunması sırasında lokalizasyon döngüsünde (`SYSTEM_INFO_LANGUAGES.forEach`) `TermsOfService_*` ve `PrivacyPolicy_*` ile alınan uyumsuzluk giderilecek. Hatalı olan `getFirstNonEmpty` mantığı lokalize keyler için doğru önceliklendirme ile yeniden yazılacak. Ayrıca `normalizedDefaults` tanımında veritabanı okumaları tam yansıtılacak.

5. **SEO Kısmındaki Gizlilik Politikası ve Kullanım Koşulları Kutusunun Kaldırılması**
   - `web/src/features/system-settings/site/seo-section.tsx` dosyasından `PrivacyPolicy_*` ve `TermsOfService_*` form alanları JSX bloklarından, `seoSchema`'dan ve `normalizedDefaults` atamalarından silinecek. İlgili `onSubmit` içindeki legal sync kodları temizlenecek.
   - `web/src/features/system-settings/site/index.tsx` içindeki varsayılan SEO yapılandırma objesinden (`PrivacyPolicy_*`, `TermsOfService_*`) çıkarılacak.

6. **Discord Bot Sorunları**
   - `service/discordbot/ownerpanel.go` dosyasında buton (`discordgo.Button`) tabanlı menüler seçilebilir menü bileşeni (`discordgo.SelectMenu`) ile değiştirilecek. Karmaşık ve eski emojili gösterimler basitleştirilecek.
   - `service/discordbot/bot.go` dosyasındaki `handleAutoReply` fonksiyonunun en başına işlenmiş mesajları (`m.ID`) kaydedip filtreleyen bir cache mekanizması (dedup) yazılacak. Böylece bir mesaja sadece tek bir kez yanıt verilmesi garantilenecek.
   - Doğrulama: Değişiklik sonrasında kodun derlenip derlenmediği `go build ./service/discordbot` ile kontrol edilecek.

7. **Hugging Face "Modal Kaydedildi" Hatası ve "Deployment Not Enabled" Hatası**
   - `web/src/features/system-settings/integrations/modal-deployment-settings-section.tsx` dosyasındaki `toast.success(t('Modal deployment settings saved successfully'))` metni `Model Deployment settings saved successfully` olarak düzeltilecek.
   - `controller/deployment.go` dosyasındaki `GetAllDeployments`, `SearchDeployments`, `GetDeployment`, `UpdateDeploymentName`, `UpdateDeployment`, `ExtendDeployment`, `DeleteDeployment`, `CreateDeployment`, `GetHardwareTypes`, `GetAvailableReplicas`, `GetPriceEstimation`, `CheckClusterNameAvailability`, `ListDeploymentContainers`, `GetContainerDetails` gibi ionet-spesifik metotlardaki `getIoEnterpriseClient` doğrulamaları olduğu gibi bırakılacak ancak genel olan `GetModelDeploymentSettings` ve `TestHuggingFaceConnection` içerisine io.net doğrulaması (`getIoAPIKey`) konulmadığından emin olunacak. `GetModelDeploymentSettings` içinde io.net kapalı olduğunda Modal ve Hugging Face bağlantı testlerinin yapılabilmesi için doğru statüler döndürülecek.

8. **Giriş ve Kayıt Ekranındaki Logo Görünümü**
   - `web/src/features/auth/auth-layout.tsx` dosyasında yer alan `<img src={logo} alt={t('Logo')} className='h-8 w-8 rounded-full object-cover' />` CSS özellikleri güncellenerek `rounded-full`, `h-8`, `w-8` sınıfları kaldırılacak, logonun orijinal oranlarında görünmesi için `className='h-auto max-h-12 w-auto object-contain'` verilecek. Skeleton `rounded-full` sınıfı da `rounded-md` olarak düzeltilecek.

9. **Mistral Test Hataları**
   - `relay/channel/mistral/adaptor.go` içerisinde yer alan `ConvertEmbeddingRequest` metodundaki `return nil, errors.New("not implemented")` ifadesi kaldırılarak `return request, nil` şeklinde çevrilecek. Böylece Mistral vb. adaptörlerde test başarısızlık hatası fırlatılmayacak.

10. **Test**
    - Run all relevant tests to ensure changes are correct and have not introduced regressions.

11. **Pre-commit**
    - Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.

12. **Submit**
    - Commit and submit the changes to the main branch.
