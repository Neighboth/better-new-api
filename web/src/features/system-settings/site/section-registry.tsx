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
import { SystemInfoSection } from '../general/system-info-section'
import {
  parseHeaderNavModules,
  parseSidebarModulesAdmin,
  serializeHeaderNavModules,
  serializeSidebarModulesAdmin,
} from '../maintenance/config'
import {
  parseCustomNavItems,
  serializeCustomNavItems,
} from '../maintenance/custom-nav-config'
import { CustomNavSection } from '../maintenance/custom-nav-section'
import { HeaderNavigationSection } from '../maintenance/header-navigation-section'
import { NoticeSection } from '../maintenance/notice-section'
import { SidebarModulesSection } from '../maintenance/sidebar-modules-section'
import type { SiteSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'
import { AdsSection } from './ads-section'
import { SEOSection } from './seo-section'

const SITE_SECTIONS = [
  {
    id: 'system-info',
    titleKey: 'System Information',
    build: (settings: SiteSettings) => (
      <SystemInfoSection
        defaultValues={{
          ...settings,
        }}
      />
    ),
  },
  {
    id: 'notice',
    titleKey: 'System Notice',
    build: (settings: SiteSettings) => (
      <NoticeSection defaultValue={settings.Notice ?? ''} />
    ),
  },
  {
    id: 'header-navigation',
    titleKey: 'Header navigation',
    build: (settings: SiteSettings) => {
      const headerNavConfig = parseHeaderNavModules(settings.HeaderNavModules)
      const headerNavSerialized = serializeHeaderNavModules(headerNavConfig)
      return (
        <HeaderNavigationSection
          config={headerNavConfig}
          initialSerialized={headerNavSerialized}
        />
      )
    },
  },
  {
    id: 'sidebar-modules',
    titleKey: 'Sidebar modules',
    build: (settings: SiteSettings) => {
      const sidebarConfig = parseSidebarModulesAdmin(
        settings.SidebarModulesAdmin
      )
      const sidebarSerialized = serializeSidebarModulesAdmin(sidebarConfig)
      return (
        <SidebarModulesSection
          config={sidebarConfig}
          initialSerialized={sidebarSerialized}
        />
      )
    },
  },
  {
    id: 'custom-navigation',
    titleKey: 'Custom navigation',
    build: (settings: SiteSettings) => {
      const customNavItems = parseCustomNavItems(settings.CustomNavItems)
      return (
        <CustomNavSection
          items={customNavItems}
          initialSerialized={serializeCustomNavItems(customNavItems)}
        />
      )
    },
  },
  {
    id: 'seo',
    titleKey: 'SEO',
    build: (settings: SiteSettings) => (
      <SEOSection
        defaultValues={{
          SEOTitlePrefix: settings.SEOTitlePrefix,
          SEODescription: settings.SEODescription,
          SEOKeywords: settings.SEOKeywords,
          SEOSocialImage: settings.SEOSocialImage,
          RobotsPolicy: settings.RobotsPolicy,
          RobotsCustomRules: settings.RobotsCustomRules,
          SitemapCustomUrls: settings.SitemapCustomUrls,
          LLMSTxt: settings.LLMSTxt,
          LLMSFullTxt: settings.LLMSFullTxt,
          GoogleAnalyticsId: settings.GoogleAnalyticsId ?? '',
          UmamiWebsiteId: settings.UmamiWebsiteId ?? '',
          UmamiScriptUrl: settings.UmamiScriptUrl ?? 'https://analytics.umami.is/script.js',
          ClarityProjectId: settings.ClarityProjectId ?? '',

          SEOTitlePrefix_tr: settings.SEOTitlePrefix_tr ?? '',
          SEOTitlePrefix_en: settings.SEOTitlePrefix_en ?? '',
          SEOTitlePrefix_zh_CN: settings.SEOTitlePrefix_zh_CN ?? '',
          SEOTitlePrefix_zh_TW: settings.SEOTitlePrefix_zh_TW ?? '',
          SEOTitlePrefix_fr: settings.SEOTitlePrefix_fr ?? '',
          SEOTitlePrefix_ru: settings.SEOTitlePrefix_ru ?? '',
          SEOTitlePrefix_ja: settings.SEOTitlePrefix_ja ?? '',
          SEOTitlePrefix_vi: settings.SEOTitlePrefix_vi ?? '',

          SEODescription_tr: settings.SEODescription_tr ?? '',
          SEODescription_en: settings.SEODescription_en ?? '',
          SEODescription_zh_CN: settings.SEODescription_zh_CN ?? '',
          SEODescription_zh_TW: settings.SEODescription_zh_TW ?? '',
          SEODescription_fr: settings.SEODescription_fr ?? '',
          SEODescription_ru: settings.SEODescription_ru ?? '',
          SEODescription_ja: settings.SEODescription_ja ?? '',
          SEODescription_vi: settings.SEODescription_vi ?? '',

          SEOKeywords_tr: settings.SEOKeywords_tr ?? '',
          SEOKeywords_en: settings.SEOKeywords_en ?? '',
          SEOKeywords_zh_CN: settings.SEOKeywords_zh_CN ?? '',
          SEOKeywords_zh_TW: settings.SEOKeywords_zh_TW ?? '',
          SEOKeywords_fr: settings.SEOKeywords_fr ?? '',
          SEOKeywords_ru: settings.SEOKeywords_ru ?? '',
          SEOKeywords_ja: settings.SEOKeywords_ja ?? '',
          SEOKeywords_vi: settings.SEOKeywords_vi ?? '',

          PrivacyPolicy_tr: settings.PrivacyPolicy_tr ?? '',
          PrivacyPolicy_en: settings.PrivacyPolicy_en ?? '',
          PrivacyPolicy_zh_CN: settings.PrivacyPolicy_zh_CN ?? '',
          PrivacyPolicy_zh_TW: settings.PrivacyPolicy_zh_TW ?? '',
          PrivacyPolicy_fr: settings.PrivacyPolicy_fr ?? '',
          PrivacyPolicy_ru: settings.PrivacyPolicy_ru ?? '',
          PrivacyPolicy_ja: settings.PrivacyPolicy_ja ?? '',
          PrivacyPolicy_vi: settings.PrivacyPolicy_vi ?? '',

          TermsOfService_tr: settings.TermsOfService_tr ?? '',
          TermsOfService_en: settings.TermsOfService_en ?? '',
          TermsOfService_zh_CN: settings.TermsOfService_zh_CN ?? '',
          TermsOfService_zh_TW: settings.TermsOfService_zh_TW ?? '',
          TermsOfService_fr: settings.TermsOfService_fr ?? '',
          TermsOfService_ru: settings.TermsOfService_ru ?? '',
          TermsOfService_ja: settings.TermsOfService_ja ?? '',
          TermsOfService_vi: settings.TermsOfService_vi ?? '',
        }}
      />
    ),
  },
  {
    id: 'ads',
    titleKey: 'Ads',
    build: (settings: SiteSettings) => (
      <AdsSection
        defaultValues={{
          AdsEnabled: settings.AdsEnabled,
          AdsMode: settings.AdsMode,
          AdSenseClientId: settings.AdSenseClientId,
          AdSenseSlotId: settings.AdSenseSlotId,
          CustomAds: settings.CustomAds,
        }}
      />
    ),
  },
] as const

export type SiteSectionId = (typeof SITE_SECTIONS)[number]['id']

const siteRegistry = createSectionRegistry<SiteSectionId, SiteSettings>({
  sections: SITE_SECTIONS,
  defaultSection: 'system-info',
  basePath: '/system-settings/site',
  urlStyle: 'path',
})

export const SITE_SECTION_IDS = siteRegistry.sectionIds
export const SITE_DEFAULT_SECTION = siteRegistry.defaultSection
export const getSiteSectionNavItems = siteRegistry.getSectionNavItems
export const getSiteSectionContent = siteRegistry.getSectionContent
export const getSiteSectionMeta = siteRegistry.getSectionMeta
