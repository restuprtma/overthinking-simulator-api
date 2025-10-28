-- CRM Module Database Migration
-- This migration creates all tables for the CRM module including:
-- - Master tables (lead_sources, sales_targets, auto_reply_rules, sales_persons)
-- - Transaction tables (leads, deals, chats, chat_messages, activities)
-- - Analytics tables (sales_performance)
-- - Materialized views for reporting

-- Create CRM schema
CREATE SCHEMA IF NOT EXISTS crm;

-- Set search path
SET search_path TO crm, public;

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- MASTER TABLES
-- ============================================================================

-- 1. CRM Company Settings Table
-- Global settings for CRM module per company
CREATE TABLE IF NOT EXISTS crm.company_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL UNIQUE,

    -- Auto-Reply Settings
    auto_reply_enabled BOOLEAN DEFAULT TRUE,
    auto_reply_global_delay_seconds INT DEFAULT 30,
    auto_reply_business_hours_start TIME DEFAULT '09:00:00',
    auto_reply_business_hours_end TIME DEFAULT '17:00:00',
    auto_reply_working_days INT[] DEFAULT ARRAY[1,2,3,4,5], -- 0=Sunday, 1=Monday, etc.

    -- Chat Settings
    chat_auto_assign_enabled BOOLEAN DEFAULT TRUE,
    chat_assignment_strategy VARCHAR(50) DEFAULT 'round_robin', -- round_robin, least_busy, manual
    chat_idle_timeout_minutes INT DEFAULT 30,

    -- Lead Settings
    lead_auto_categorization_enabled BOOLEAN DEFAULT TRUE,
    lead_hot_criteria JSONB, -- AI criteria for hot leads
    lead_warm_criteria JSONB,
    lead_cold_criteria JSONB,

    -- AI Settings
    ai_enabled BOOLEAN DEFAULT TRUE,
    ai_confidence_threshold DECIMAL(3,2) DEFAULT 0.70, -- Minimum confidence to auto-respond
    ai_training_mode BOOLEAN DEFAULT FALSE, -- If true, all responses need approval

    -- Notification Settings
    notifications_enabled BOOLEAN DEFAULT TRUE,
    notification_channels JSONB, -- {email: true, whatsapp: true, telegram: false}

    -- Timezone
    timezone VARCHAR(50) DEFAULT 'Asia/Jakarta',

    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID
);

-- 2. Lead Sources Table
-- Master data for lead/prospect sources (WhatsApp, Website, Referral, etc.)
CREATE TABLE IF NOT EXISTS crm.lead_sources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL,
    description TEXT,
    color VARCHAR(7),
    icon VARCHAR(50),
    is_active BOOLEAN DEFAULT TRUE,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 3. Sales Targets Table
-- Monthly sales targets (team and individual)
CREATE TABLE IF NOT EXISTS crm.sales_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    year INT NOT NULL,
    month INT NOT NULL CHECK (month >= 1 AND month <= 12),
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('team', 'individual')),
    company_user_id UUID, -- References core.company_users(id) - for individual targets
    target_amount DECIMAL(15,2) NOT NULL,
    achieved_amount DECIMAL(15,2) DEFAULT 0,
    achievement_percentage DECIMAL(5,2) DEFAULT 0,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 4. Auto Reply Rules Table
-- Configuration for WhatsApp auto-reply rules and AI training data
CREATE TABLE IF NOT EXISTS crm.auto_reply_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_enabled BOOLEAN DEFAULT TRUE,
    trigger_type VARCHAR(50) NOT NULL CHECK (trigger_type IN (
        'first_message',           -- New contact - AI training data
        'outside_business_hours',  -- After hours - AI response
        'keyword_match',           -- Keyword detection - AI analysis
        'no_sales_available',      -- Sales unavailable - AI takeover
        'high_priority_lead',      -- High priority lead - AI alert
        'ai_confidence_low',       -- Low AI confidence - Human handoff
        'conversation_flow'        -- Conversation flow - AI learning
    )),
    conditions JSONB,              -- Dynamic conditions based on trigger_type:
                                   -- - keywords: string[] for keyword_match
                                   -- - timeRange: {start, end} for outside_business_hours
                                   -- - salesAssigned: UUID[] for specific sales assignment
                                   -- - leadStatus: string[] for lead status filtering
                                   -- - minConfidenceScore: number for AI confidence threshold
    message_template TEXT NOT NULL,
    delay_seconds INT DEFAULT 0,   -- Delay before sending auto-reply
    priority INT DEFAULT 1,        -- Lower number = higher priority
    usage_count INT DEFAULT 0,     -- Track how many times rule was triggered
    last_used_at TIMESTAMP,        -- Last time rule was used
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 5. Sales Persons Table
-- Stores sales/marketing representatives information for each company
-- A sales person is a specialized role of a company user
CREATE TABLE IF NOT EXISTS crm.sales_persons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_user_id UUID NOT NULL REFERENCES core.company_users(id) ON DELETE RESTRICT,
    sales_code VARCHAR(50) NOT NULL,
    sales_name VARCHAR(255), -- Optional: Display name for sales (can differ from user full_name)
    sales_area VARCHAR(100), -- Sales territory/area coverage
    sales_target DECIMAL(15,2), -- Monthly or annual sales target
    commission_rate DECIMAL(5,2), -- Commission percentage (e.g., 5.00 for 5%)
    whatsapp VARCHAR(20), -- WhatsApp number for sales person
    is_whatsapp_connected BOOLEAN DEFAULT FALSE, -- WhatsApp connection status
    is_active BOOLEAN DEFAULT TRUE,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- ============================================================================
-- TRANSACTION TABLES
-- ============================================================================

-- 6. Leads Table
-- Main table for prospect/lead management
CREATE TABLE IF NOT EXISTS crm.leads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    lead_number VARCHAR(50) UNIQUE,
    name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(255),
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(255),
    category VARCHAR(20) NOT NULL CHECK (category IN ('hot', 'warm', 'cold')),
    status VARCHAR(50) NOT NULL CHECK (status IN ('new', 'in_progress', 'follow_up', 'negotiation', 'won', 'lost')),
    source_id UUID NOT NULL,
    assigned_to_company_user_id UUID, -- References core.company_users(id) - user assigned to this lead
    deal_value DECIMAL(15,2),
    last_contact_at TIMESTAMP,
    next_follow_up_at TIMESTAMP,
    response_time_minutes INT,
    ai_highlights JSONB,
    notes TEXT,
    address TEXT,
    city VARCHAR(100),
    province VARCHAR(100),
    postal_code VARCHAR(10),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 7. Deals Table
-- Closed deals (won/lost) for revenue tracking
CREATE TABLE IF NOT EXISTS crm.deals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    deal_number VARCHAR(50) UNIQUE NOT NULL,
    lead_id UUID,
    client_name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(255),
    phone VARCHAR(20),
    email VARCHAR(255),
    sales_person_id UUID NOT NULL, -- References crm.sales_persons(id) - sales person who closed this deal
    deal_value DECIMAL(15,2) NOT NULL,
    commission DECIMAL(15,2),
    commission_percentage DECIMAL(5,2) DEFAULT 5.00,
    status VARCHAR(20) NOT NULL CHECK (status IN ('won', 'lost')),
    category VARCHAR(50),
    source_id UUID,
    close_date DATE NOT NULL,
    duration_days INT,
    notes TEXT,
    loss_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 8. Chats Table
-- Chat sessions with customers (WhatsApp, etc.)
CREATE TABLE IF NOT EXISTS crm.chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    lead_id UUID,
    customer_name VARCHAR(255) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(255),
    assigned_to_company_user_id UUID, -- References core.company_users(id) - user assigned to this chat
    platform VARCHAR(50) NOT NULL CHECK (platform IN ('whatsapp', 'telegram', 'email', 'phone')),
    status VARCHAR(50) NOT NULL CHECK (status IN ('active', 'archived', 'closed')),
    category VARCHAR(20) CHECK (category IN ('hot', 'warm', 'cold')),
    last_message_at TIMESTAMP,
    last_message_preview TEXT,
    last_message_sender VARCHAR(20) CHECK (last_message_sender IN ('customer', 'sales', 'system')),
    unread_count INT DEFAULT 0,
    message_count INT DEFAULT 0,
    ai_insights JSONB,
    avg_response_time_minutes INT,
    first_response_time_minutes INT,
    sentiment_score DECIMAL(3,2),
    tags TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- 9. Chat Messages Table
-- Individual messages in chat conversations
CREATE TABLE IF NOT EXISTS crm.chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL,
    sender_type VARCHAR(20) NOT NULL CHECK (sender_type IN ('customer', 'sales', 'system', 'auto_reply')),
    sender_id UUID,
    sender_name VARCHAR(255),
    content TEXT NOT NULL,
    message_type VARCHAR(20) NOT NULL CHECK (message_type IN ('text', 'image', 'video', 'file', 'voice', 'location')),
    media_url VARCHAR(500),
    media_mime_type VARCHAR(100),
    media_size_bytes BIGINT,
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMP,
    is_sent BOOLEAN DEFAULT TRUE,
    sent_at TIMESTAMP NOT NULL,
    delivery_status VARCHAR(20) CHECK (delivery_status IN ('sent', 'delivered', 'read', 'failed')),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 10. Auto Reply Logs Table
-- Track auto-reply rule execution and AI performance
CREATE TABLE IF NOT EXISTS crm.auto_reply_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    auto_reply_rule_id UUID NOT NULL REFERENCES crm.auto_reply_rules(id) ON DELETE CASCADE,
    chat_id UUID, -- References crm.chats(id)
    lead_id UUID, -- References crm.leads(id)
    customer_phone VARCHAR(20) NOT NULL,
    customer_name VARCHAR(255),
    trigger_type VARCHAR(50) NOT NULL,
    matched_condition JSONB, -- What condition/keyword triggered this rule
    message_sent TEXT NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    was_manual_override BOOLEAN DEFAULT FALSE, -- If sales manually intervened
    sales_response_time_minutes INT, -- Time until sales responded after auto-reply
    customer_replied BOOLEAN DEFAULT FALSE,
    customer_reply_time_minutes INT,
    ai_confidence_score DECIMAL(3,2), -- AI confidence in this response (0.00-1.00)
    effectiveness_score DECIMAL(3,2), -- Calculated effectiveness based on customer response
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 11. Activities Table
-- Activity timeline and sales actions tracking
CREATE TABLE IF NOT EXISTS crm.activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    lead_id UUID,
    deal_id UUID,
    chat_id UUID,
    company_user_id UUID NOT NULL, -- References core.company_users(id) - user who performed the activity
    activity_type VARCHAR(50) NOT NULL CHECK (activity_type IN ('call', 'meeting', 'email', 'note', 'task', 'follow_up', 'demo', 'proposal')),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    activity_date TIMESTAMP NOT NULL,
    duration_minutes INT,
    outcome VARCHAR(100) CHECK (outcome IN ('successful', 'no_answer', 'callback', 'not_interested', 'interested', 'scheduled')),
    next_action TEXT,
    reminder_at TIMESTAMP,
    is_completed BOOLEAN DEFAULT TRUE,
    completed_at TIMESTAMP,
    priority VARCHAR(20) CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by UUID,
    deleted_at TIMESTAMP,
    deleted_by UUID
);

-- ============================================================================
-- ANALYTICS TABLES
-- ============================================================================

-- 12. Sales Performance Table
-- Aggregated performance metrics per sales person per period
CREATE TABLE IF NOT EXISTS crm.sales_performance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    company_user_id UUID NOT NULL, -- References core.company_users(id) - user whose performance is tracked
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    period_type VARCHAR(20) NOT NULL CHECK (period_type IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly')),
    follow_ups_completed INT DEFAULT 0,
    follow_ups_target INT DEFAULT 0,
    deals_closed INT DEFAULT 0,
    deals_target INT DEFAULT 0,
    deals_lost INT DEFAULT 0,
    revenue_generated DECIMAL(15,2) DEFAULT 0,
    revenue_target DECIMAL(15,2) DEFAULT 0,
    commission_earned DECIMAL(15,2) DEFAULT 0,
    leads_created INT DEFAULT 0,
    leads_converted INT DEFAULT 0,
    avg_response_time_minutes INT,
    first_response_time_minutes INT,
    conversion_rate DECIMAL(5,2) DEFAULT 0,
    win_rate DECIMAL(5,2) DEFAULT 0,
    calls_made INT DEFAULT 0,
    meetings_held INT DEFAULT 0,
    proposals_sent INT DEFAULT 0,
    emails_sent INT DEFAULT 0,
    performance_score INT DEFAULT 0,
    rank INT,
    badge VARCHAR(50) CHECK (badge IN ('top_performer', 'high_performer', 'average_performer', 'needs_improvement')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- UNIQUE CONSTRAINTS
-- ============================================================================

-- Lead sources: unique code per company
CREATE UNIQUE INDEX uq_lead_sources_company_code
ON crm.lead_sources(company_id, code)
WHERE deleted_at IS NULL;

-- Sales persons: unique company_user_id (one sales record per company user)
CREATE UNIQUE INDEX uq_sales_persons_company_user_id
ON crm.sales_persons(company_user_id)
WHERE deleted_at IS NULL;

-- Sales targets: unique period per company and target type
CREATE UNIQUE INDEX uq_sales_targets_period
ON crm.sales_targets(company_id, year, month, target_type, COALESCE(company_user_id, '00000000-0000-0000-0000-000000000000'::UUID))
WHERE deleted_at IS NULL;

-- Leads: unique phone per company
CREATE UNIQUE INDEX uq_leads_company_phone
ON crm.leads(company_id, phone)
WHERE deleted_at IS NULL;

-- Deals: unique deal number per company
CREATE UNIQUE INDEX uq_deals_deal_number
ON crm.deals(company_id, deal_number)
WHERE deleted_at IS NULL;

-- Chats: unique phone per platform per company
CREATE UNIQUE INDEX uq_chats_company_phone
ON crm.chats(company_id, phone, platform)
WHERE deleted_at IS NULL;

-- Sales performance: unique period per user
CREATE UNIQUE INDEX uq_sales_performance_period
ON crm.sales_performance(company_id, company_user_id, period_start, period_end, period_type);

-- ============================================================================
-- INDEXES - COMPANY SETTINGS
-- ============================================================================

CREATE INDEX idx_company_settings_company_id ON crm.company_settings(company_id);
CREATE INDEX idx_company_settings_auto_reply_enabled ON crm.company_settings(auto_reply_enabled);
CREATE INDEX idx_company_settings_ai_enabled ON crm.company_settings(ai_enabled);

-- ============================================================================
-- INDEXES - LEAD SOURCES
-- ============================================================================

CREATE INDEX idx_lead_sources_company_id ON crm.lead_sources(company_id);
CREATE INDEX idx_lead_sources_code ON crm.lead_sources(code);
CREATE INDEX idx_lead_sources_is_active ON crm.lead_sources(is_active);
CREATE INDEX idx_lead_sources_deleted_at ON crm.lead_sources(deleted_at);
CREATE INDEX idx_lead_sources_sort_order ON crm.lead_sources(sort_order);

-- ============================================================================
-- INDEXES - SALES TARGETS
-- ============================================================================

CREATE INDEX idx_sales_targets_company_id ON crm.sales_targets(company_id);
CREATE INDEX idx_sales_targets_company_user_id ON crm.sales_targets(company_user_id);
CREATE INDEX idx_sales_targets_year ON crm.sales_targets(year);
CREATE INDEX idx_sales_targets_month ON crm.sales_targets(month);
CREATE INDEX idx_sales_targets_year_month ON crm.sales_targets(year, month);
CREATE INDEX idx_sales_targets_target_type ON crm.sales_targets(target_type);
CREATE INDEX idx_sales_targets_deleted_at ON crm.sales_targets(deleted_at);

-- ============================================================================
-- INDEXES - AUTO REPLY RULES
-- ============================================================================

CREATE INDEX idx_auto_reply_rules_company_id ON crm.auto_reply_rules(company_id);
CREATE INDEX idx_auto_reply_rules_is_enabled ON crm.auto_reply_rules(is_enabled);
CREATE INDEX idx_auto_reply_rules_trigger_type ON crm.auto_reply_rules(trigger_type);
CREATE INDEX idx_auto_reply_rules_priority ON crm.auto_reply_rules(priority);
CREATE INDEX idx_auto_reply_rules_deleted_at ON crm.auto_reply_rules(deleted_at);

-- ============================================================================
-- INDEXES - SALES PERSONS
-- ============================================================================

CREATE INDEX idx_sales_persons_company_user_id ON crm.sales_persons(company_user_id);
CREATE INDEX idx_sales_persons_sales_code ON crm.sales_persons(sales_code);
CREATE INDEX idx_sales_persons_is_active ON crm.sales_persons(is_active);
CREATE INDEX idx_sales_persons_deleted_at ON crm.sales_persons(deleted_at);
CREATE INDEX idx_sales_persons_sales_area ON crm.sales_persons(sales_area);
CREATE INDEX idx_sales_persons_whatsapp ON crm.sales_persons(whatsapp);
CREATE INDEX idx_sales_persons_is_whatsapp_connected ON crm.sales_persons(is_whatsapp_connected);

-- ============================================================================
-- INDEXES - LEADS
-- ============================================================================

CREATE INDEX idx_leads_company_id ON crm.leads(company_id);
CREATE INDEX idx_leads_lead_number ON crm.leads(lead_number);
CREATE INDEX idx_leads_assigned_to_company_user_id ON crm.leads(assigned_to_company_user_id);
CREATE INDEX idx_leads_category ON crm.leads(category);
CREATE INDEX idx_leads_status ON crm.leads(status);
CREATE INDEX idx_leads_source_id ON crm.leads(source_id);
CREATE INDEX idx_leads_phone ON crm.leads(phone);
CREATE INDEX idx_leads_email ON crm.leads(email);
CREATE INDEX idx_leads_next_follow_up_at ON crm.leads(next_follow_up_at);
CREATE INDEX idx_leads_last_contact_at ON crm.leads(last_contact_at);
CREATE INDEX idx_leads_deleted_at ON crm.leads(deleted_at);
CREATE INDEX idx_leads_created_at ON crm.leads(created_at);

-- ============================================================================
-- INDEXES - DEALS
-- ============================================================================

CREATE INDEX idx_deals_company_id ON crm.deals(company_id);
CREATE INDEX idx_deals_deal_number ON crm.deals(deal_number);
CREATE INDEX idx_deals_lead_id ON crm.deals(lead_id);
CREATE INDEX idx_deals_sales_person_id ON crm.deals(sales_person_id);
CREATE INDEX idx_deals_status ON crm.deals(status);
CREATE INDEX idx_deals_category ON crm.deals(category);
CREATE INDEX idx_deals_source_id ON crm.deals(source_id);
CREATE INDEX idx_deals_close_date ON crm.deals(close_date);
CREATE INDEX idx_deals_deal_value ON crm.deals(deal_value);
CREATE INDEX idx_deals_deleted_at ON crm.deals(deleted_at);
CREATE INDEX idx_deals_created_at ON crm.deals(created_at);

-- ============================================================================
-- INDEXES - CHATS
-- ============================================================================

CREATE INDEX idx_chats_company_id ON crm.chats(company_id);
CREATE INDEX idx_chats_lead_id ON crm.chats(lead_id);
CREATE INDEX idx_chats_assigned_to_company_user_id ON crm.chats(assigned_to_company_user_id);
CREATE INDEX idx_chats_phone ON crm.chats(phone);
CREATE INDEX idx_chats_email ON crm.chats(email);
CREATE INDEX idx_chats_platform ON crm.chats(platform);
CREATE INDEX idx_chats_status ON crm.chats(status);
CREATE INDEX idx_chats_category ON crm.chats(category);
CREATE INDEX idx_chats_last_message_at ON crm.chats(last_message_at);
CREATE INDEX idx_chats_unread_count ON crm.chats(unread_count);
CREATE INDEX idx_chats_deleted_at ON crm.chats(deleted_at);

-- ============================================================================
-- INDEXES - CHAT MESSAGES
-- ============================================================================

CREATE INDEX idx_chat_messages_chat_id ON crm.chat_messages(chat_id);
CREATE INDEX idx_chat_messages_sender_id ON crm.chat_messages(sender_id);
CREATE INDEX idx_chat_messages_sender_type ON crm.chat_messages(sender_type);
CREATE INDEX idx_chat_messages_message_type ON crm.chat_messages(message_type);
CREATE INDEX idx_chat_messages_sent_at ON crm.chat_messages(sent_at);
CREATE INDEX idx_chat_messages_is_read ON crm.chat_messages(is_read);
CREATE INDEX idx_chat_messages_delivery_status ON crm.chat_messages(delivery_status);

-- ============================================================================
-- INDEXES - AUTO REPLY LOGS
-- ============================================================================

CREATE INDEX idx_auto_reply_logs_company_id ON crm.auto_reply_logs(company_id);
CREATE INDEX idx_auto_reply_logs_auto_reply_rule_id ON crm.auto_reply_logs(auto_reply_rule_id);
CREATE INDEX idx_auto_reply_logs_chat_id ON crm.auto_reply_logs(chat_id);
CREATE INDEX idx_auto_reply_logs_lead_id ON crm.auto_reply_logs(lead_id);
CREATE INDEX idx_auto_reply_logs_customer_phone ON crm.auto_reply_logs(customer_phone);
CREATE INDEX idx_auto_reply_logs_trigger_type ON crm.auto_reply_logs(trigger_type);
CREATE INDEX idx_auto_reply_logs_sent_at ON crm.auto_reply_logs(sent_at);
CREATE INDEX idx_auto_reply_logs_was_manual_override ON crm.auto_reply_logs(was_manual_override);
CREATE INDEX idx_auto_reply_logs_customer_replied ON crm.auto_reply_logs(customer_replied);

-- ============================================================================
-- INDEXES - ACTIVITIES
-- ============================================================================

CREATE INDEX idx_activities_company_id ON crm.activities(company_id);
CREATE INDEX idx_activities_lead_id ON crm.activities(lead_id);
CREATE INDEX idx_activities_deal_id ON crm.activities(deal_id);
CREATE INDEX idx_activities_chat_id ON crm.activities(chat_id);
CREATE INDEX idx_activities_company_user_id ON crm.activities(company_user_id);
CREATE INDEX idx_activities_activity_type ON crm.activities(activity_type);
CREATE INDEX idx_activities_activity_date ON crm.activities(activity_date);
CREATE INDEX idx_activities_reminder_at ON crm.activities(reminder_at);
CREATE INDEX idx_activities_is_completed ON crm.activities(is_completed);
CREATE INDEX idx_activities_priority ON crm.activities(priority);
CREATE INDEX idx_activities_deleted_at ON crm.activities(deleted_at);

-- ============================================================================
-- INDEXES - SALES PERFORMANCE
-- ============================================================================

CREATE INDEX idx_sales_performance_company_id ON crm.sales_performance(company_id);
CREATE INDEX idx_sales_performance_company_user_id ON crm.sales_performance(company_user_id);
CREATE INDEX idx_sales_performance_period_start ON crm.sales_performance(period_start);
CREATE INDEX idx_sales_performance_period_end ON crm.sales_performance(period_end);
CREATE INDEX idx_sales_performance_period_type ON crm.sales_performance(period_type);
CREATE INDEX idx_sales_performance_performance_score ON crm.sales_performance(performance_score);
CREATE INDEX idx_sales_performance_rank ON crm.sales_performance(rank);
CREATE INDEX idx_sales_performance_period_range ON crm.sales_performance(company_id, period_start, period_end);

-- ============================================================================
-- COMPOSITE INDEXES FOR COMMON QUERIES
-- ============================================================================

-- Lead filtering by company, status, and assigned sales
CREATE INDEX idx_leads_company_status_assigned
ON crm.leads(company_id, status, assigned_to_company_user_id)
WHERE deleted_at IS NULL;

-- Deal filtering for revenue reports
CREATE INDEX idx_deals_company_date_status
ON crm.deals(company_id, close_date, status)
WHERE deleted_at IS NULL;

-- Chat filtering for sales inbox
CREATE INDEX idx_chats_company_assigned_status
ON crm.chats(company_id, assigned_to_company_user_id, status)
WHERE deleted_at IS NULL;

-- Activity filtering for timeline
CREATE INDEX idx_activities_lead_date
ON crm.activities(lead_id, activity_date DESC)
WHERE deleted_at IS NULL;

-- Auto-reply logs for analytics
CREATE INDEX idx_auto_reply_logs_company_rule_date
ON crm.auto_reply_logs(company_id, auto_reply_rule_id, sent_at DESC);

-- ============================================================================
-- PARTIAL INDEXES FOR PERFORMANCE OPTIMIZATION
-- ============================================================================

-- Active leads only (exclude won/lost)
CREATE INDEX idx_leads_active
ON crm.leads(company_id, assigned_to_company_user_id, category)
WHERE deleted_at IS NULL AND status NOT IN ('won', 'lost');

-- Unread chats
CREATE INDEX idx_chats_unread
ON crm.chats(company_id, assigned_to_company_user_id)
WHERE deleted_at IS NULL AND unread_count > 0;

-- Upcoming follow-ups (removed CURRENT_TIMESTAMP from WHERE clause as it's not immutable)
CREATE INDEX idx_leads_upcoming_followup
ON crm.leads(company_id, assigned_to_company_user_id, next_follow_up_at)
WHERE deleted_at IS NULL;

-- Won deals for revenue calculation
CREATE INDEX idx_deals_won_revenue
ON crm.deals(company_id, sales_person_id, close_date, deal_value)
WHERE deleted_at IS NULL AND status = 'won';

-- ============================================================================
-- MATERIALIZED VIEWS
-- ============================================================================

-- Dashboard Statistics View
CREATE MATERIALIZED VIEW IF NOT EXISTS crm.mv_dashboard_stats AS
SELECT
    company_id,
    COUNT(DISTINCT CASE WHEN deleted_at IS NULL THEN id END) as total_leads,
    COUNT(DISTINCT CASE WHEN deleted_at IS NULL AND category = 'hot' THEN id END) as hot_leads,
    COUNT(DISTINCT CASE WHEN deleted_at IS NULL AND category = 'warm' THEN id END) as warm_leads,
    COUNT(DISTINCT CASE WHEN deleted_at IS NULL AND category = 'cold' THEN id END) as cold_leads,
    COUNT(DISTINCT CASE WHEN deleted_at IS NULL AND status = 'follow_up' AND next_follow_up_at::date = CURRENT_DATE THEN id END) as follow_ups_today,
    AVG(CASE WHEN deleted_at IS NULL THEN response_time_minutes END) as avg_response_time
FROM crm.leads
GROUP BY company_id;

CREATE UNIQUE INDEX idx_mv_dashboard_stats_company ON crm.mv_dashboard_stats(company_id);

-- Revenue by Source View
CREATE MATERIALIZED VIEW IF NOT EXISTS crm.mv_revenue_by_source AS
SELECT
    d.company_id,
    ls.name as source_name,
    DATE_TRUNC('month', d.close_date) as month,
    COUNT(*) as deals_count,
    SUM(CASE WHEN d.status = 'won' THEN d.deal_value ELSE 0 END) as total_revenue,
    AVG(CASE WHEN d.status = 'won' THEN d.deal_value END) as avg_deal_value,
    CASE
        WHEN COUNT(*) > 0 THEN (SUM(CASE WHEN d.status = 'won' THEN 1 ELSE 0 END)::float / COUNT(*) * 100)
        ELSE 0
    END as win_rate
FROM crm.deals d
LEFT JOIN crm.lead_sources ls ON d.source_id = ls.id
WHERE d.deleted_at IS NULL
GROUP BY d.company_id, ls.name, DATE_TRUNC('month', d.close_date);

CREATE INDEX idx_mv_revenue_by_source_company ON crm.mv_revenue_by_source(company_id);
CREATE INDEX idx_mv_revenue_by_source_month ON crm.mv_revenue_by_source(month);

-- Auto-Reply Performance View
CREATE MATERIALIZED VIEW IF NOT EXISTS crm.mv_auto_reply_performance AS
SELECT
    ar.company_id,
    ar.id as rule_id,
    ar.name as rule_name,
    ar.trigger_type,
    COUNT(arl.id) as total_executions,
    COUNT(CASE WHEN arl.customer_replied THEN 1 END) as customer_replied_count,
    CASE
        WHEN COUNT(arl.id) > 0 THEN (COUNT(CASE WHEN arl.customer_replied THEN 1 END)::float / COUNT(arl.id) * 100)
        ELSE 0
    END as reply_rate,
    AVG(arl.customer_reply_time_minutes) as avg_customer_reply_time_minutes,
    AVG(arl.sales_response_time_minutes) as avg_sales_response_time_minutes,
    AVG(arl.ai_confidence_score) as avg_ai_confidence,
    AVG(arl.effectiveness_score) as avg_effectiveness,
    COUNT(CASE WHEN arl.was_manual_override THEN 1 END) as manual_override_count,
    COUNT(CASE WHEN DATE(arl.sent_at) = CURRENT_DATE THEN 1 END) as executions_today,
    COUNT(CASE WHEN arl.sent_at >= CURRENT_DATE - INTERVAL '7 days' THEN 1 END) as executions_last_7_days,
    COUNT(CASE WHEN arl.sent_at >= CURRENT_DATE - INTERVAL '30 days' THEN 1 END) as executions_last_30_days,
    MAX(arl.sent_at) as last_executed_at
FROM crm.auto_reply_rules ar
LEFT JOIN crm.auto_reply_logs arl ON ar.id = arl.auto_reply_rule_id
WHERE ar.deleted_at IS NULL
GROUP BY ar.company_id, ar.id, ar.name, ar.trigger_type;

CREATE UNIQUE INDEX idx_mv_auto_reply_performance_rule ON crm.mv_auto_reply_performance(rule_id);
CREATE INDEX idx_mv_auto_reply_performance_company ON crm.mv_auto_reply_performance(company_id);
CREATE INDEX idx_mv_auto_reply_performance_trigger ON crm.mv_auto_reply_performance(trigger_type);
