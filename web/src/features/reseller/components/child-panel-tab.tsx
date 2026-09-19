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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  AlertCircle,
  CheckCircle2,
  CreditCard,
  Eye,
  EyeOff,
  Globe,
  Image,
  Info,
  Loader2,
  Save,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import { updateResellerConfig } from '../api'
import type { ResellerConfig } from '../types'

interface ChildPanelTabProps {
  config: ResellerConfig
}

export function ChildPanelTab({ config }: ChildPanelTabProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  // Domain & Branding
  const [customDomain, setCustomDomain] = useState(config.custom_domain || '')
  const [siteName, setSiteName] = useState(config.site_name || '')
  const [logoUrl, setLogoUrl] = useState(config.logo || config.logo_url || '')
  const [faviconUrl, setFaviconUrl] = useState(config.favicon || config.favicon_url || '')
  const [seoTitle, setSeoTitle] = useState(config.seo_title || '')
  const [seoDescription, setSeoDescription] = useState(config.seo_description || '')
  const [seoKeywords, setSeoKeywords] = useState(config.seo_keywords || '')
  const [homePageContent, setHomePageContent] = useState(
    config.homepage_content || config.home_page_content || ''
  )

  // Payment Gateways
  const [epayPartnerId, setEpayPartnerId] = useState(config.epay_partner_id || '')
  const [epayKey, setEpayKey] = useState(config.epay_key || config.epay_partner_key || '')
  const [epayUrl, setEpayUrl] = useState(config.epay_url || config.epay_gateway_url || '')
  const [epayCallbackUrl, setEpayCallbackUrl] = useState(config.epay_callback_url || '')

  const [stripeApiSecret, setStripeApiSecret] = useState(config.stripe_api_secret || '')
  const [stripeWebhookSecret, setStripeWebhookSecret] = useState(
    config.stripe_webhook_secret || ''
  )
  const [stripePriceId, setStripePriceId] = useState(config.stripe_price_id || '')

  const [creemApiKey, setCreemApiKey] = useState(config.creem_api_key || '')
  const [creemWebhookSecret, setCreemWebhookSecret] = useState(
    config.creem_webhook_secret || ''
  )
  const [creemTestMode, setCreemTestMode] = useState<boolean>(
    Boolean(config.creem_test_mode)
  )

  const [waffoMerchantId, setWaffoMerchantId] = useState(config.waffo_merchant_id || '')
  const [waffoApiKey, setWaffoApiKey] = useState(config.waffo_api_key || '')
  const [waffoPrivateKey, setWaffoPrivateKey] = useState(config.waffo_private_key || '')

  // 5 New Gateways
  const [shopierApiKey, setShopierApiKey] = useState(config.shopier_api_key || '')
  const [shopierApiSecret, setShopierApiSecret] = useState(config.shopier_api_secret || '')
  const [shopierWebsiteIndex, setShopierWebsiteIndex] = useState(config.shopier_website_index || '1')

  const [paytrMerchantId, setPayTRMerchantId] = useState(config.paytr_merchant_id || '')
  const [paytrMerchantKey, setPayTRMerchantKey] = useState(config.paytr_merchant_key || '')
  const [paytrMerchantSalt, setPayTRMerchantSalt] = useState(config.paytr_merchant_salt || '')
  const [paytrTestMode, setPayTRTestMode] = useState<boolean>(Boolean(config.paytr_test_mode))

  const [paypalClientId, setPayPalClientId] = useState(config.paypal_client_id || '')
  const [paypalClientSecret, setPayPalClientSecret] = useState(config.paypal_client_secret || '')
  const [paypalMode, setPayPalMode] = useState(config.paypal_mode || 'sandbox')

  const [iyzicoApiKey, setIyzicoApiKey] = useState(config.iyzico_api_key || '')
  const [iyzicoSecretKey, setIyzicoSecretKey] = useState(config.iyzico_secret_key || '')
  const [iyzicoBaseUrl, setIyzicoBaseUrl] = useState(config.iyzico_base_url || 'https://sandbox-api.iyzipay.com')

  const [shopifyStoreDomain, setShopifyStoreDomain] = useState(config.shopify_store_domain || '')
  const [shopifyAccessToken, setShopifyAccessToken] = useState(config.shopify_access_token || '')
  const [shopifyWebhookSecret, setShopifyWebhookSecret] = useState(config.shopify_webhook_secret || '')

  // Visibility toggles for sensitive secret keys
  const [showEpayKey, setShowEpayKey] = useState(false)
  const [showStripeSecret, setShowStripeSecret] = useState(false)
  const [showCreemKey, setShowCreemKey] = useState(false)
  const [showWaffoKey, setShowWaffoKey] = useState(false)
  const [showShopierSecret, setShowShopierSecret] = useState(false)
  const [showPayTRKey, setShowPayTRKey] = useState(false)
  const [showPayPalSecret, setShowPayPalSecret] = useState(false)
  const [showIyzicoSecret, setShowIyzicoSecret] = useState(false)
  const [showShopifyToken, setShowShopifyToken] = useState(false)

  useEffect(() => {
    setCustomDomain(config.custom_domain || '')
    setSiteName(config.site_name || '')
    setLogoUrl(config.logo || config.logo_url || '')
    setFaviconUrl(config.favicon || config.favicon_url || '')
    setSeoTitle(config.seo_title || '')
    setSeoDescription(config.seo_description || '')
    setSeoKeywords(config.seo_keywords || '')
    setHomePageContent(config.homepage_content || config.home_page_content || '')

    setEpayPartnerId(config.epay_partner_id || '')
    setEpayKey(config.epay_key || config.epay_partner_key || '')
    setEpayUrl(config.epay_url || config.epay_gateway_url || '')
    setEpayCallbackUrl(config.epay_callback_url || '')

    setStripeApiSecret(config.stripe_api_secret || '')
    setStripeWebhookSecret(config.stripe_webhook_secret || '')
    setStripePriceId(config.stripe_price_id || '')

    setCreemApiKey(config.creem_api_key || '')
    setCreemWebhookSecret(config.creem_webhook_secret || '')
    setCreemTestMode(Boolean(config.creem_test_mode))

    setWaffoMerchantId(config.waffo_merchant_id || '')
    setWaffoApiKey(config.waffo_api_key || '')
    setWaffoPrivateKey(config.waffo_private_key || '')

    setShopierApiKey(config.shopier_api_key || '')
    setShopierApiSecret(config.shopier_api_secret || '')
    setShopierWebsiteIndex(config.shopier_website_index || '1')

    setPayTRMerchantId(config.paytr_merchant_id || '')
    setPayTRMerchantKey(config.paytr_merchant_key || '')
    setPayTRMerchantSalt(config.paytr_merchant_salt || '')
    setPayTRTestMode(Boolean(config.paytr_test_mode))

    setPayPalClientId(config.paypal_client_id || '')
    setPayPalClientSecret(config.paypal_client_secret || '')
    setPayPalMode(config.paypal_mode || 'sandbox')

    setIyzicoApiKey(config.iyzico_api_key || '')
    setIyzicoSecretKey(config.iyzico_secret_key || '')
    setIyzicoBaseUrl(config.iyzico_base_url || 'https://sandbox-api.iyzipay.com')

    setShopifyStoreDomain(config.shopify_store_domain || '')
    setShopifyAccessToken(config.shopify_access_token || '')
    setShopifyWebhookSecret(config.shopify_webhook_secret || '')
  }, [config])

  const saveMutation = useMutation({
    mutationFn: async () => {
      return updateResellerConfig({
        custom_domain: customDomain.trim(),
        site_name: siteName.trim(),
        logo: logoUrl.trim(),
        logo_url: logoUrl.trim(),
        favicon: faviconUrl.trim(),
        favicon_url: faviconUrl.trim(),
        seo_title: seoTitle.trim(),
        seo_description: seoDescription.trim(),
        seo_keywords: seoKeywords.trim(),
        homepage_content: homePageContent,
        home_page_content: homePageContent,
        // Payment gateways
        epay_partner_id: epayPartnerId.trim(),
        epay_key: epayKey.trim(),
        epay_partner_key: epayKey.trim(),
        epay_url: epayUrl.trim(),
        epay_gateway_url: epayUrl.trim(),
        epay_callback_url: epayCallbackUrl.trim(),
        stripe_api_secret: stripeApiSecret.trim(),
        stripe_webhook_secret: stripeWebhookSecret.trim(),
        stripe_price_id: stripePriceId.trim(),
        creem_api_key: creemApiKey.trim(),
        creem_webhook_secret: creemWebhookSecret.trim(),
        creem_test_mode: creemTestMode,
        waffo_merchant_id: waffoMerchantId.trim(),
        waffo_api_key: waffoApiKey.trim(),
        waffo_private_key: waffoPrivateKey.trim(),
        shopier_api_key: shopierApiKey.trim(),
        shopier_api_secret: shopierApiSecret.trim(),
        shopier_website_index: shopierWebsiteIndex.trim(),
        paytr_merchant_id: paytrMerchantId.trim(),
        paytr_merchant_key: paytrMerchantKey.trim(),
        paytr_merchant_salt: paytrMerchantSalt.trim(),
        paytr_test_mode: paytrTestMode,
        paypal_client_id: paypalClientId.trim(),
        paypal_client_secret: paypalClientSecret.trim(),
        paypal_mode: paypalMode.trim(),
        iyzico_api_key: iyzicoApiKey.trim(),
        iyzico_secret_key: iyzicoSecretKey.trim(),
        iyzico_base_url: iyzicoBaseUrl.trim(),
        shopify_store_domain: shopifyStoreDomain.trim(),
        shopify_access_token: shopifyAccessToken.trim(),
        shopify_webhook_secret: shopifyWebhookSecret.trim(),
      })
    },
    onSuccess: () => {
      toast.success(t('Child panel settings saved successfully'))
      queryClient.invalidateQueries({ queryKey: ['reseller-self-config'] })
    },
    onError: (err: any) => {
      toast.error(err.message || t('Failed to save child panel settings'))
    },
  })

  const isEnabled = config.child_panel_enabled

  return (
    <div className='space-y-6'>
      {!isEnabled ? (
        <Alert variant='destructive' className='border-amber-500/50 bg-amber-500/10 text-amber-700 dark:text-amber-400'>
          <AlertCircle className='h-5 w-5' />
          <AlertTitle className='font-semibold'>{t('Child Panel Not Enabled')}</AlertTitle>
          <AlertDescription className='text-xs mt-1 leading-relaxed'>
            {t(
              'The child panel feature is currently disabled for your reseller account. Please contact the platform administrator to activate child panel capability before deploying your custom domain and branding.'
            )}
          </AlertDescription>
        </Alert>
      ) : (
        <Alert className='border-emerald-500/50 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'>
          <CheckCircle2 className='h-5 w-5' />
          <AlertTitle className='font-semibold'>{t('Child Panel Active & Configured')}</AlertTitle>
          <AlertDescription className='text-xs mt-1 leading-relaxed'>
            {t(
              "Your child panel is ready! Point your CNAME record to this server, enter your custom domain below, and save your custom branding."
            )}
          </AlertDescription>
        </Alert>
      )}

      {/* Domain & Brand Info */}
      <Card>
        <CardHeader>
          <CardTitle className='text-base flex items-center gap-2'>
            <Globe className='h-4 w-4 text-primary' />
            {t('Domain & Branding Configuration')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Customize the domain, name, logos, and SEO properties for your child panel portal.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='custom_domain' className='text-xs font-medium'>
                {t('Custom Domain (CNAME)')}
              </Label>
              <Input
                id='custom_domain'
                placeholder='ai.yourdomain.com'
                value={customDomain}
                onChange={(e) => setCustomDomain(e.target.value)}
                disabled={!isEnabled || saveMutation.isPending}
              />
              <p className='text-[11px] text-muted-foreground'>
                {t("Point a CNAME record from your domain to this server's host. Do not include https://.")}
              </p>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='site_name' className='text-xs font-medium'>
                {t('Portal Site Name')}
              </Label>
              <Input
                id='site_name'
                placeholder='My AI Platform'
                value={siteName}
                onChange={(e) => setSiteName(e.target.value)}
                disabled={!isEnabled || saveMutation.isPending}
              />
              <p className='text-[11px] text-muted-foreground'>
                {t('The display title of your platform in navigation bars, footers, and emails.')}
              </p>
            </div>
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='logo_url' className='text-xs font-medium'>
                {t('Logo URL')}
              </Label>
              <div className='flex gap-2 items-center'>
                <Input
                  id='logo_url'
                  placeholder='https://example.com/logo.png'
                  value={logoUrl}
                  onChange={(e) => setLogoUrl(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
                {logoUrl && (
                  <div className='h-9 w-9 shrink-0 overflow-hidden rounded border bg-muted flex items-center justify-center'>
                    <img
                      src={logoUrl}
                      alt='Logo preview'
                      className='h-full w-full object-contain'
                      onError={(e) => {
                        ;(e.currentTarget as HTMLImageElement).style.display = 'none'
                      }}
                    />
                  </div>
                )}
              </div>
              <p className='text-[11px] text-muted-foreground'>
                {t('Direct link to your site logo image (PNG, SVG, or WEBP recommended).')}
              </p>
            </div>

            <div className='space-y-1.5'>
              <Label htmlFor='favicon_url' className='text-xs font-medium'>
                {t('Favicon URL')}
              </Label>
              <div className='flex gap-2 items-center'>
                <Input
                  id='favicon_url'
                  placeholder='https://example.com/favicon.ico'
                  value={faviconUrl}
                  onChange={(e) => setFaviconUrl(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
                {faviconUrl && (
                  <div className='h-9 w-9 shrink-0 overflow-hidden rounded border bg-muted flex items-center justify-center'>
                    <img
                      src={faviconUrl}
                      alt='Favicon preview'
                      className='h-4 w-4 object-contain'
                      onError={(e) => {
                        ;(e.currentTarget as HTMLImageElement).style.display = 'none'
                      }}
                    />
                  </div>
                )}
              </div>
              <p className='text-[11px] text-muted-foreground'>
                {t('Direct link to your 32x32 or 64x64 favicon icon.')}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* SEO Configuration */}
      <Card>
        <CardHeader>
          <CardTitle className='text-base flex items-center gap-2'>
            <Info className='h-4 w-4 text-primary' />
            {t('Search Engine Optimization (SEO)')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Configure search engine titles, snippets, and indexing keywords for your panel.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-1.5'>
            <Label htmlFor='seo_title' className='text-xs font-medium'>
              {t('SEO Page Title')}
            </Label>
            <Input
              id='seo_title'
              placeholder='Fastest AI API Gateway | Your Brand'
              value={seoTitle}
              onChange={(e) => setSeoTitle(e.target.value)}
              disabled={!isEnabled || saveMutation.isPending}
            />
            <p className='text-[11px] text-muted-foreground'>
              {t('Shown in browser tabs and as the primary link headline in search results.')}
            </p>
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='seo_description' className='text-xs font-medium'>
              {t('SEO Meta Description')}
            </Label>
            <Textarea
              id='seo_description'
              rows={2}
              placeholder='Direct access to 100+ LLMs with 99.9% uptime.'
              value={seoDescription}
              onChange={(e) => setSeoDescription(e.target.value)}
              disabled={!isEnabled || saveMutation.isPending}
            />
            <p className='text-[11px] text-muted-foreground'>
              {t('Brief summary snippet displayed below your link in Google search results.')}
            </p>
          </div>

          <div className='space-y-1.5'>
            <Label htmlFor='seo_keywords' className='text-xs font-medium'>
              {t('SEO Keywords')}
            </Label>
            <Input
              id='seo_keywords'
              placeholder='ai gateway, openai api, claude api, llm'
              value={seoKeywords}
              onChange={(e) => setSeoKeywords(e.target.value)}
              disabled={!isEnabled || saveMutation.isPending}
            />
            <p className='text-[11px] text-muted-foreground'>
              {t('Comma-separated keywords for search engine indexing.')}
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Homepage Content */}
      <Card>
        <CardHeader>
          <CardTitle className='text-base flex items-center gap-2'>
            <Image className='h-4 w-4 text-primary' />
            {t('Homepage Hero & Notice Content')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t("Custom Markdown or HTML displayed on your child panel's public homepage.")}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <Textarea
            rows={6}
            placeholder={t('Enter custom HTML or markdown for your portal landing page...')}
            value={homePageContent}
            onChange={(e) => setHomePageContent(e.target.value)}
            disabled={!isEnabled || saveMutation.isPending}
          />
        </CardContent>
      </Card>

      {/* Payment Gateways Section (Child Panel Only) */}
      <Card>
        <CardHeader>
          <CardTitle className='text-base flex items-center gap-2'>
            <CreditCard className='h-4 w-4 text-primary' />
            {t('Child Panel Payment Gateways')}
          </CardTitle>
          <CardDescription className='text-xs'>
            {t('Configure personal payment gateway credentials for top-ups conducted on your child panel.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue='epay' className='space-y-4'>
            <div className='overflow-x-auto pb-1'>
              <TabsList className='inline-flex min-w-full justify-start md:grid md:grid-cols-9'>
                <TabsTrigger value='epay'>EPay</TabsTrigger>
                <TabsTrigger value='stripe'>Stripe</TabsTrigger>
                <TabsTrigger value='creem'>Creem</TabsTrigger>
                <TabsTrigger value='waffo'>Waffo</TabsTrigger>
                <TabsTrigger value='shopier'>Shopier</TabsTrigger>
                <TabsTrigger value='paytr'>PayTR</TabsTrigger>
                <TabsTrigger value='paypal'>PayPal</TabsTrigger>
                <TabsTrigger value='iyzico'>iyzico</TabsTrigger>
                <TabsTrigger value='shopify'>Shopify</TabsTrigger>
              </TabsList>
            </div>

            {/* EPay Gateway */}
            <TabsContent value='epay' className='space-y-4 pt-2'>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='epay_url' className='text-xs font-medium'>
                    {t('EPay Gateway URL (PayAddress)')}
                  </Label>
                  <Input
                    id='epay_url'
                    placeholder='https://pay.example.com'
                    value={epayUrl}
                    onChange={(e) => setEpayUrl(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='epay_callback_url' className='text-xs font-medium'>
                    {t('Custom Callback Address')}
                  </Label>
                  <Input
                    id='epay_callback_url'
                    placeholder='https://api.yourdomain.com'
                    value={epayCallbackUrl}
                    onChange={(e) => setEpayCallbackUrl(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='epay_partner_id' className='text-xs font-medium'>
                    {t('EPay Partner ID (EpayId)')}
                  </Label>
                  <Input
                    id='epay_partner_id'
                    placeholder='1000'
                    value={epayPartnerId}
                    onChange={(e) => setEpayPartnerId(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='epay_key' className='text-xs font-medium'>
                    {t('EPay Key (EpayKey)')}
                  </Label>
                  <div className='relative'>
                    <Input
                      id='epay_key'
                      type={showEpayKey ? 'text' : 'password'}
                      placeholder='secret key'
                      value={epayKey}
                      onChange={(e) => setEpayKey(e.target.value)}
                      disabled={!isEnabled || saveMutation.isPending}
                      className='pr-10'
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                      onClick={() => setShowEpayKey(!showEpayKey)}
                    >
                      {showEpayKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                    </Button>
                  </div>
                </div>
              </div>
            </TabsContent>

            {/* Stripe Gateway */}
            <TabsContent value='stripe' className='space-y-4 pt-2'>
              <div className='space-y-1.5'>
                <Label htmlFor='stripe_api_secret' className='text-xs font-medium'>
                  {t('Stripe Secret Key')}
                </Label>
                <div className='relative'>
                  <Input
                    id='stripe_api_secret'
                    type={showStripeSecret ? 'text' : 'password'}
                    placeholder='sk_live_...'
                    value={stripeApiSecret}
                    onChange={(e) => setStripeApiSecret(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowStripeSecret(!showStripeSecret)}
                  >
                    {showStripeSecret ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='stripe_webhook_secret' className='text-xs font-medium'>
                    {t('Stripe Webhook Secret')}
                  </Label>
                  <Input
                    id='stripe_webhook_secret'
                    placeholder='whsec_...'
                    value={stripeWebhookSecret}
                    onChange={(e) => setStripeWebhookSecret(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='stripe_price_id' className='text-xs font-medium'>
                    {t('Stripe Price ID')}
                  </Label>
                  <Input
                    id='stripe_price_id'
                    placeholder='price_...'
                    value={stripePriceId}
                    onChange={(e) => setStripePriceId(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
              </div>
            </TabsContent>

            {/* Creem Gateway */}
            <TabsContent value='creem' className='space-y-4 pt-2'>
              <div className='space-y-1.5'>
                <Label htmlFor='creem_api_key' className='text-xs font-medium'>
                  {t('Creem API Key')}
                </Label>
                <div className='relative'>
                  <Input
                    id='creem_api_key'
                    type={showCreemKey ? 'text' : 'password'}
                    placeholder='creem_api_...'
                    value={creemApiKey}
                    onChange={(e) => setCreemApiKey(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowCreemKey(!showCreemKey)}
                  >
                    {showCreemKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='creem_webhook_secret' className='text-xs font-medium'>
                  {t('Creem Webhook Secret')}
                </Label>
                <Input
                  id='creem_webhook_secret'
                  placeholder='creem_whsec_...'
                  value={creemWebhookSecret}
                  onChange={(e) => setCreemWebhookSecret(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
              </div>
              <div className='flex items-center space-x-2 pt-1'>
                <Switch
                  id='creem_test_mode'
                  checked={creemTestMode}
                  onCheckedChange={setCreemTestMode}
                  disabled={!isEnabled || saveMutation.isPending}
                />
                <Label htmlFor='creem_test_mode' className='text-xs font-medium cursor-pointer'>
                  {t('Creem Test Mode')}
                </Label>
              </div>
            </TabsContent>

            {/* Waffo Gateway */}
            <TabsContent value='waffo' className='space-y-4 pt-2'>
              <div className='space-y-1.5'>
                <Label htmlFor='waffo_merchant_id' className='text-xs font-medium'>
                  {t('Waffo Merchant ID')}
                </Label>
                <Input
                  id='waffo_merchant_id'
                  placeholder='Merchant ID'
                  value={waffoMerchantId}
                  onChange={(e) => setWaffoMerchantId(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='waffo_api_key' className='text-xs font-medium'>
                  {t('Waffo API Key')}
                </Label>
                <div className='relative'>
                  <Input
                    id='waffo_api_key'
                    type={showWaffoKey ? 'text' : 'password'}
                    placeholder='Waffo API Key'
                    value={waffoApiKey}
                    onChange={(e) => setWaffoApiKey(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowWaffoKey(!showWaffoKey)}
                  >
                    {showWaffoKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='waffo_private_key' className='text-xs font-medium'>
                  {t('Waffo Private Key')}
                </Label>
                <Textarea
                  id='waffo_private_key'
                  rows={3}
                  placeholder='-----BEGIN PRIVATE KEY----- ...'
                  value={waffoPrivateKey}
                  onChange={(e) => setWaffoPrivateKey(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                  className='font-mono text-xs'
                />
              </div>
            </TabsContent>

            {/* Shopier Gateway */}
            <TabsContent value='shopier' className='space-y-4 pt-2'>
              <div className='rounded-md bg-amber-50 p-3 text-xs text-amber-900 dark:bg-amber-950 dark:text-amber-100'>
                <p className='font-medium'>{t('Shopier Webhook / Callback:')}</p>
                <code className='font-mono'>{'<ChildDomain>/api/user/shopier/notify'}</code>
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='shopier_api_key' className='text-xs font-medium'>
                    {t('Shopier API Key')}
                  </Label>
                  <Input
                    id='shopier_api_key'
                    placeholder='Shopier API Key'
                    value={shopierApiKey}
                    onChange={(e) => setShopierApiKey(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='shopier_website_index' className='text-xs font-medium'>
                    {t('Website Index')}
                  </Label>
                  <Input
                    id='shopier_website_index'
                    placeholder='1'
                    value={shopierWebsiteIndex}
                    onChange={(e) => setShopierWebsiteIndex(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='shopier_api_secret' className='text-xs font-medium'>
                  {t('Shopier API Secret')}
                </Label>
                <div className='relative'>
                  <Input
                    id='shopier_api_secret'
                    type={showShopierSecret ? 'text' : 'password'}
                    placeholder='Shopier API Secret'
                    value={shopierApiSecret}
                    onChange={(e) => setShopierApiSecret(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowShopierSecret(!showShopierSecret)}
                  >
                    {showShopierSecret ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
            </TabsContent>

            {/* PayTR Gateway */}
            <TabsContent value='paytr' className='space-y-4 pt-2'>
              <div className='rounded-md bg-blue-50 p-3 text-xs text-blue-900 dark:bg-blue-950 dark:text-blue-100'>
                <p className='font-medium'>{t('PayTR Notification URL:')}</p>
                <code className='font-mono'>{'<ChildDomain>/api/user/paytr/notify'}</code>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='paytr_merchant_id' className='text-xs font-medium'>
                  {t('PayTR Merchant ID')}
                </Label>
                <Input
                  id='paytr_merchant_id'
                  placeholder='Merchant ID'
                  value={paytrMerchantId}
                  onChange={(e) => setPayTRMerchantId(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='paytr_merchant_key' className='text-xs font-medium'>
                    {t('PayTR Merchant Key')}
                  </Label>
                  <div className='relative'>
                    <Input
                      id='paytr_merchant_key'
                      type={showPayTRKey ? 'text' : 'password'}
                      placeholder='Merchant Key'
                      value={paytrMerchantKey}
                      onChange={(e) => setPayTRMerchantKey(e.target.value)}
                      disabled={!isEnabled || saveMutation.isPending}
                      className='pr-10'
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                      onClick={() => setShowPayTRKey(!showPayTRKey)}
                    >
                      {showPayTRKey ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                    </Button>
                  </div>
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='paytr_merchant_salt' className='text-xs font-medium'>
                    {t('PayTR Merchant Salt')}
                  </Label>
                  <Input
                    id='paytr_merchant_salt'
                    type='password'
                    placeholder='Merchant Salt'
                    value={paytrMerchantSalt}
                    onChange={(e) => setPayTRMerchantSalt(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
              </div>
              <div className='flex items-center space-x-2 pt-1'>
                <Switch
                  id='paytr_test_mode'
                  checked={paytrTestMode}
                  onCheckedChange={setPayTRTestMode}
                  disabled={!isEnabled || saveMutation.isPending}
                />
                <Label htmlFor='paytr_test_mode' className='text-xs font-medium cursor-pointer'>
                  {t('PayTR Test Mode')}
                </Label>
              </div>
            </TabsContent>

            {/* PayPal Gateway */}
            <TabsContent value='paypal' className='space-y-4 pt-2'>
              <div className='rounded-md bg-sky-50 p-3 text-xs text-sky-900 dark:bg-sky-950 dark:text-sky-100'>
                <p className='font-medium'>{t('PayPal Return / Capture URL:')}</p>
                <code className='font-mono'>{'<ChildDomain>/api/user/paypal/capture'}</code>
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='paypal_client_id' className='text-xs font-medium'>
                    {t('PayPal Client ID')}
                  </Label>
                  <Input
                    id='paypal_client_id'
                    placeholder='Client ID'
                    value={paypalClientId}
                    onChange={(e) => setPayPalClientId(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='paypal_mode' className='text-xs font-medium'>
                    {t('PayPal Mode')}
                  </Label>
                  <Select
                    value={paypalMode || 'sandbox'}
                    onValueChange={(val) => setPayPalMode(val || 'sandbox')}
                    disabled={!isEnabled || saveMutation.isPending}
                  >
                    <SelectTrigger id='paypal_mode'>
                      <SelectValue placeholder={t('Select Environment')} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='sandbox'>
                        {t('Sandbox (Test)')}
                      </SelectItem>
                      <SelectItem value='live'>
                        {t('Production (Live)')}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='paypal_client_secret' className='text-xs font-medium'>
                  {t('PayPal Client Secret')}
                </Label>
                <div className='relative'>
                  <Input
                    id='paypal_client_secret'
                    type={showPayPalSecret ? 'text' : 'password'}
                    placeholder='Client Secret'
                    value={paypalClientSecret}
                    onChange={(e) => setPayPalClientSecret(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowPayPalSecret(!showPayPalSecret)}
                  >
                    {showPayPalSecret ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
            </TabsContent>

            {/* iyzico Gateway */}
            <TabsContent value='iyzico' className='space-y-4 pt-2'>
              <div className='rounded-md bg-blue-50 p-3 text-xs text-blue-900 dark:bg-blue-950 dark:text-blue-100'>
                <p className='font-medium'>{t('iyzico Callback URL:')}</p>
                <code className='font-mono'>{'<ChildDomain>/api/user/iyzico/callback'}</code>
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='iyzico_api_key' className='text-xs font-medium'>
                    {t('iyzico API Key')}
                  </Label>
                  <Input
                    id='iyzico_api_key'
                    placeholder='API Key'
                    value={iyzicoApiKey}
                    onChange={(e) => setIyzicoApiKey(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='iyzico_base_url' className='text-xs font-medium'>
                    {t('Environment / Base URL')}
                  </Label>
                  <Select
                    value={iyzicoBaseUrl || 'https://sandbox-api.iyzipay.com'}
                    onValueChange={(val) => setIyzicoBaseUrl(val || 'https://sandbox-api.iyzipay.com')}
                    disabled={!isEnabled || saveMutation.isPending}
                  >
                    <SelectTrigger id='iyzico_base_url'>
                      <SelectValue placeholder={t('Select Environment')} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='https://sandbox-api.iyzipay.com'>
                        {t('Sandbox (Test)')} - https://sandbox-api.iyzipay.com
                      </SelectItem>
                      <SelectItem value='https://api.iyzipay.com'>
                        {t('Production (Live)')} - https://api.iyzipay.com
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='iyzico_secret_key' className='text-xs font-medium'>
                  {t('iyzico Secret Key')}
                </Label>
                <div className='relative'>
                  <Input
                    id='iyzico_secret_key'
                    type={showIyzicoSecret ? 'text' : 'password'}
                    placeholder='Secret Key'
                    value={iyzicoSecretKey}
                    onChange={(e) => setIyzicoSecretKey(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                    className='pr-10'
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                    onClick={() => setShowIyzicoSecret(!showIyzicoSecret)}
                  >
                    {showIyzicoSecret ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                  </Button>
                </div>
              </div>
            </TabsContent>

            {/* Shopify Gateway */}
            <TabsContent value='shopify' className='space-y-4 pt-2'>
              <div className='rounded-md bg-emerald-50 p-3 text-xs text-emerald-900 dark:bg-emerald-950 dark:text-emerald-100'>
                <p className='font-medium'>{t('Shopify Webhook URL:')}</p>
                <code className='font-mono'>{'<ChildDomain>/api/user/shopify/webhook'}</code>
              </div>
              <div className='space-y-1.5'>
                <Label htmlFor='shopify_store_domain' className='text-xs font-medium'>
                  {t('Shopify Store Domain')}
                </Label>
                <Input
                  id='shopify_store_domain'
                  placeholder='your-store.myshopify.com'
                  value={shopifyStoreDomain}
                  onChange={(e) => setShopifyStoreDomain(e.target.value)}
                  disabled={!isEnabled || saveMutation.isPending}
                />
              </div>
              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-1.5'>
                  <Label htmlFor='shopify_access_token' className='text-xs font-medium'>
                    {t('Admin API Access Token')}
                  </Label>
                  <div className='relative'>
                    <Input
                      id='shopify_access_token'
                      type={showShopifyToken ? 'text' : 'password'}
                      placeholder='shpat_...'
                      value={shopifyAccessToken}
                      onChange={(e) => setShopifyAccessToken(e.target.value)}
                      disabled={!isEnabled || saveMutation.isPending}
                      className='pr-10'
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='absolute right-0 top-0 h-full px-3 text-muted-foreground hover:text-foreground'
                      onClick={() => setShowShopifyToken(!showShopifyToken)}
                    >
                      {showShopifyToken ? <EyeOff className='h-4 w-4' /> : <Eye className='h-4 w-4' />}
                    </Button>
                  </div>
                </div>
                <div className='space-y-1.5'>
                  <Label htmlFor='shopify_webhook_secret' className='text-xs font-medium'>
                    {t('Webhook HMAC Secret')}
                  </Label>
                  <Input
                    id='shopify_webhook_secret'
                    type='password'
                    placeholder='shpss_...'
                    value={shopifyWebhookSecret}
                    onChange={(e) => setShopifyWebhookSecret(e.target.value)}
                    disabled={!isEnabled || saveMutation.isPending}
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>

      {isEnabled && (
        <div className='flex justify-end'>
          <Button
            onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <Save className='mr-2 h-4 w-4' />
            )}
            {t('Save Child Panel Settings')}
          </Button>
        </div>
      )}
    </div>
  )
}
