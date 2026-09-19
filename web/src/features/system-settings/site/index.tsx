/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { SettingsPage } from '../components/settings-page'
import type { SiteSettings } from '../types'
import {
  SITE_DEFAULT_SECTION,
  getSiteSectionContent,
  getSiteSectionMeta,
} from './section-registry.tsx'

const defaultSiteSettings: SiteSettings = {
  Notice: '',
  SystemName: 'New API',
  Logo: '',
  Footer: '',
  About: '',
  HomePageContent: '',
  ServerAddress: '',
  'legal.user_agreement': '',
  'legal.privacy_policy': '',
  HeaderNavModules: '',
  SidebarModulesAdmin: '',
  CustomNavItems: '',
  SEOTitlePrefix: '',
  SEODescription: '',
  SEOKeywords: '',
  SEOSocialImage: '',
  RobotsPolicy: 'allow_all',
  RobotsCustomRules: '',
  SitemapCustomUrls: '',
  LLMSTxt: '',
  LLMSFullTxt: '',
  BlogEnabled: false,
  AdSenseClientId: '',
  AdSenseSlotId: '',
  AdsEnabled: false,
  AdsMode: 'both',
  CustomAds: '[]',
  GoogleAnalyticsId: '',
  UmamiWebsiteId: '',
  UmamiScriptUrl: '',
  ClarityProjectId: '',
  // Multilingual SEO (8 languages)
  SEOTitlePrefix_tr: '',
  SEOTitlePrefix_en: '',
  SEOTitlePrefix_zh_CN: '',
  SEOTitlePrefix_zh_TW: '',
  SEOTitlePrefix_fr: '',
  SEOTitlePrefix_ru: '',
  SEOTitlePrefix_ja: '',
  SEOTitlePrefix_vi: '',

  SEODescription_tr: '',
  SEODescription_en: '',
  SEODescription_zh_CN: '',
  SEODescription_zh_TW: '',
  SEODescription_fr: '',
  SEODescription_ru: '',
  SEODescription_ja: '',
  SEODescription_vi: '',

  SEOKeywords_tr: '',
  SEOKeywords_en: '',
  SEOKeywords_zh_CN: '',
  SEOKeywords_zh_TW: '',
  SEOKeywords_fr: '',
  SEOKeywords_ru: '',
  SEOKeywords_ja: '',
  SEOKeywords_vi: '',

  PrivacyPolicy_tr: '',
  PrivacyPolicy_en: '',
  PrivacyPolicy_zh_CN: '',
  PrivacyPolicy_zh_TW: '',
  PrivacyPolicy_fr: '',
  PrivacyPolicy_ru: '',
  PrivacyPolicy_ja: '',
  PrivacyPolicy_vi: '',

  TermsOfService_tr: '',
  TermsOfService_en: '',
  TermsOfService_zh_CN: '',
  TermsOfService_zh_TW: '',
  TermsOfService_fr: '',
  TermsOfService_ru: '',
  TermsOfService_ja: '',
  TermsOfService_vi: '',

  // Multilingual System Info (8 languages)
  SystemName_tr: '',
  SystemName_en: '',
  SystemName_zh_CN: '',
  SystemName_zh_TW: '',
  SystemName_fr: '',
  SystemName_ru: '',
  SystemName_ja: '',
  SystemName_vi: '',

  Footer_tr: '',
  Footer_en: '',
  Footer_zh_CN: '',
  Footer_zh_TW: '',
  Footer_fr: '',
  Footer_ru: '',
  Footer_ja: '',
  Footer_vi: '',

  About_tr: '',
  About_en: '',
  About_zh_CN: '',
  About_zh_TW: '',
  About_fr: '',
  About_ru: '',
  About_ja: '',
  About_vi: '',

  HomePageContent_tr: '',
  HomePageContent_en: '',
  HomePageContent_zh_CN: '',
  HomePageContent_zh_TW: '',
  HomePageContent_fr: '',
  HomePageContent_ru: '',
  HomePageContent_ja: '',
  HomePageContent_vi: '',

  'legal.user_agreement_tr': '',
  'legal.user_agreement_en': '',
  'legal.user_agreement_zh_CN': '',
  'legal.user_agreement_zh_TW': '',
  'legal.user_agreement_fr': '',
  'legal.user_agreement_ru': '',
  'legal.user_agreement_ja': '',
  'legal.user_agreement_vi': '',

  'legal.privacy_policy_tr': '',
  'legal.privacy_policy_en': '',
  'legal.privacy_policy_zh_CN': '',
  'legal.privacy_policy_zh_TW': '',
  'legal.privacy_policy_fr': '',
  'legal.privacy_policy_ru': '',
  'legal.privacy_policy_ja': '',
  'legal.privacy_policy_vi': '',
}

export function SiteSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/site/$section'
      defaultSettings={defaultSiteSettings}
      defaultSection={SITE_DEFAULT_SECTION}
      getSectionContent={getSiteSectionContent}
      getSectionMeta={getSiteSectionMeta}
    />
  )
}
