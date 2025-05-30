// 工具函数库

/**
 * 防抖函数
 * @param {Function} func 要防抖的函数
 * @param {number} wait 等待时间（毫秒）
 * @param {boolean} immediate 是否立即执行
 * @returns {Function} 防抖后的函数
 */
function debounce(func, wait, immediate = false) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            timeout = null;
            if (!immediate) func.apply(this, args);
        };
        const callNow = immediate && !timeout;
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
        if (callNow) func.apply(this, args);
    };
}

/**
 * 节流函数
 * @param {Function} func 要节流的函数
 * @param {number} limit 限制时间（毫秒）
 * @returns {Function} 节流后的函数
 */
function throttle(func, limit) {
    let inThrottle;
    return function (...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

/**
 * 获取元素相对于视口的位置
 * @param {Element} element DOM元素
 * @returns {Object} 位置信息
 */
function getElementViewportPosition(element) {
    const rect = element.getBoundingClientRect();
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
    const viewportWidth = window.innerWidth || document.documentElement.clientWidth;

    return {
        top: rect.top,
        bottom: rect.bottom,
        left: rect.left,
        right: rect.right,
        width: rect.width,
        height: rect.height,
        centerX: rect.left + rect.width / 2,
        centerY: rect.top + rect.height / 2,
        isInViewport: rect.top < viewportHeight && rect.bottom > 0,
        visibilityRatio: Math.max(0, Math.min(1,
            (Math.min(rect.bottom, viewportHeight) - Math.max(rect.top, 0)) / rect.height
        ))
    };
}

/**
 * 平滑滚动到指定元素
 * @param {Element|string} target 目标元素或选择器
 * @param {Object} options 选项
 */
function smoothScrollTo(target, options = {}) {
    const element = typeof target === 'string' ? document.querySelector(target) : target;
    if (!element) return;

    const {
        offset = 0,
        duration = 800,
        easing = 'easeInOutCubic'
    } = options;

    const targetPosition = element.offsetTop - offset;
    const startPosition = window.pageYOffset;
    const distance = targetPosition - startPosition;
    let startTime = null;

    const easingFunctions = {
        linear: t => t,
        easeInQuad: t => t * t,
        easeOutQuad: t => t * (2 - t),
        easeInOutQuad: t => t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t,
        easeInCubic: t => t * t * t,
        easeOutCubic: t => (--t) * t * t + 1,
        easeInOutCubic: t => t < 0.5 ? 4 * t * t * t : (t - 1) * (2 * t - 2) * (2 * t - 2) + 1
    };

    function animation(currentTime) {
        if (startTime === null) startTime = currentTime;
        const timeElapsed = currentTime - startTime;
        const progress = Math.min(timeElapsed / duration, 1);
        const easedProgress = easingFunctions[easing](progress);

        window.scrollTo(0, startPosition + distance * easedProgress);

        if (timeElapsed < duration) {
            requestAnimationFrame(animation);
        }
    }

    requestAnimationFrame(animation);
}

/**
 * 数字动画计数器
 * @param {Element} element 目标元素
 * @param {number} target 目标数字
 * @param {Object} options 选项
 */
function animateNumber(element, target, options = {}) {
    const {
        duration = 2000,
        decimals = 0,
        separator = ',',
        prefix = '',
        suffix = '',
        easing = 'easeOutCubic'
    } = options;

    const start = parseFloat(element.textContent.replace(/[^\d.-]/g, '')) || 0;
    const difference = target - start;
    let startTime = null;

    const easingFunctions = {
        linear: t => t,
        easeOutCubic: t => (--t) * t * t + 1,
        easeInOutCubic: t => t < 0.5 ? 4 * t * t * t : (t - 1) * (2 * t - 2) * (2 * t - 2) + 1
    };

    function step(currentTime) {
        if (startTime === null) startTime = currentTime;
        const timeElapsed = currentTime - startTime;
        const progress = Math.min(timeElapsed / duration, 1);
        const easedProgress = easingFunctions[easing](progress);

        const current = start + difference * easedProgress;
        const formatted = formatNumber(current, decimals, separator);
        element.textContent = prefix + formatted + suffix;

        if (progress < 1) {
            requestAnimationFrame(step);
        }
    }

    requestAnimationFrame(step);
}

/**
 * 格式化数字
 * @param {number} num 数字
 * @param {number} decimals 小数位数
 * @param {string} separator 千分位分隔符
 * @returns {string} 格式化后的数字字符串
 */
function formatNumber(num, decimals = 0, separator = ',') {
    const fixed = parseFloat(num).toFixed(decimals);
    const parts = fixed.split('.');
    parts[0] = parts[0].replace(/\B(?=(\d{3})+(?!\d))/g, separator);
    return parts.join('.');
}

/**
 * 创建SVG元素
 * @param {string} tagName 标签名
 * @param {Object} attributes 属性对象
 * @returns {SVGElement} SVG元素
 */
function createSVGElement(tagName, attributes = {}) {
    const element = document.createElementNS('http://www.w3.org/2000/svg', tagName);
    Object.entries(attributes).forEach(([key, value]) => {
        element.setAttribute(key, value);
    });
    return element;
}

/**
 * 随机数生成器
 * @param {number} min 最小值
 * @param {number} max 最大值
 * @param {boolean} integer 是否返回整数
 * @returns {number} 随机数
 */
function random(min, max, integer = false) {
    const rand = Math.random() * (max - min) + min;
    return integer ? Math.floor(rand) : rand;
}

/**
 * 生成随机颜色
 * @param {string} type 颜色类型 ('hex', 'rgb', 'hsl')
 * @returns {string} 颜色字符串
 */
function randomColor(type = 'hex') {
    switch (type) {
        case 'hex':
            return '#' + Math.floor(Math.random() * 16777215).toString(16).padStart(6, '0');
        case 'rgb':
            return `rgb(${random(0, 255, true)}, ${random(0, 255, true)}, ${random(0, 255, true)})`;
        case 'hsl':
            return `hsl(${random(0, 360, true)}, ${random(50, 100, true)}%, ${random(40, 80, true)}%)`;
        default:
            return randomColor('hex');
    }
}

/**
 * 延迟执行函数
 * @param {number} ms 延迟时间（毫秒）
 * @returns {Promise} Promise对象
 */
function delay(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * 检查元素是否在视口内
 * @param {Element} element DOM元素
 * @param {number} threshold 阈值（0-1）
 * @returns {boolean} 是否在视口内
 */
function isInViewport(element, threshold = 0) {
    const rect = element.getBoundingClientRect();
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
    const viewportWidth = window.innerWidth || document.documentElement.clientWidth;

    const verticalVisible = rect.top < viewportHeight && rect.bottom > 0;
    const horizontalVisible = rect.left < viewportWidth && rect.right > 0;

    if (!verticalVisible || !horizontalVisible) return false;

    if (threshold === 0) return true;

    const visibleArea = Math.max(0, Math.min(rect.bottom, viewportHeight) - Math.max(rect.top, 0)) *
        Math.max(0, Math.min(rect.right, viewportWidth) - Math.max(rect.left, 0));
    const totalArea = rect.width * rect.height;

    return (visibleArea / totalArea) >= threshold;
}

/**
 * 类型检查工具
 */
const typeCheck = {
    isString: value => typeof value === 'string',
    isNumber: value => typeof value === 'number' && !isNaN(value),
    isBoolean: value => typeof value === 'boolean',
    isFunction: value => typeof value === 'function',
    isArray: value => Array.isArray(value),
    isObject: value => value !== null && typeof value === 'object' && !Array.isArray(value),
    isNull: value => value === null,
    isUndefined: value => value === undefined,
    isEmpty: value => {
        if (value === null || value === undefined) return true;
        if (typeof value === 'string' || Array.isArray(value)) return value.length === 0;
        if (typeof value === 'object') return Object.keys(value).length === 0;
        return false;
    }
};

/**
 * DOM操作工具
 */
const dom = {
    /**
     * 添加类名
     * @param {Element} element DOM元素
     * @param {string|Array} className 类名
     */
    addClass: (element, className) => {
        if (Array.isArray(className)) {
            element.classList.add(...className);
        } else {
            element.classList.add(className);
        }
    },

    /**
     * 移除类名
     * @param {Element} element DOM元素
     * @param {string|Array} className 类名
     */
    removeClass: (element, className) => {
        if (Array.isArray(className)) {
            element.classList.remove(...className);
        } else {
            element.classList.remove(className);
        }
    },

    /**
     * 切换类名
     * @param {Element} element DOM元素
     * @param {string} className 类名
     */
    toggleClass: (element, className) => {
        element.classList.toggle(className);
    },

    /**
     * 检查是否包含类名
     * @param {Element} element DOM元素
     * @param {string} className 类名
     * @returns {boolean} 是否包含
     */
    hasClass: (element, className) => {
        return element.classList.contains(className);
    },

    /**
     * 查找元素
     * @param {string} selector 选择器
     * @param {Element} context 上下文元素
     * @returns {Element|null} 找到的元素
     */
    find: (selector, context = document) => {
        return context.querySelector(selector);
    },

    /**
     * 查找所有元素
     * @param {string} selector 选择器
     * @param {Element} context 上下文元素
     * @returns {NodeList} 找到的元素列表
     */
    findAll: (selector, context = document) => {
        return context.querySelectorAll(selector);
    },

    /**
     * 创建元素
     * @param {string} tagName 标签名
     * @param {Object} attributes 属性对象
     * @param {string} textContent 文本内容
     * @returns {Element} 创建的元素
     */
    create: (tagName, attributes = {}, textContent = '') => {
        const element = document.createElement(tagName);
        Object.entries(attributes).forEach(([key, value]) => {
            if (key === 'className') {
                element.className = value;
            } else if (key === 'innerHTML') {
                element.innerHTML = value;
            } else {
                element.setAttribute(key, value);
            }
        });
        if (textContent) element.textContent = textContent;
        return element;
    }
};

/**
 * 事件工具
 */
const events = {
    /**
     * 添加事件监听器
     * @param {Element|string} element 元素或选择器
     * @param {string} event 事件类型
     * @param {Function} handler 事件处理器
     * @param {Object} options 选项
     */
    on: (element, event, handler, options = {}) => {
        const el = typeof element === 'string' ? document.querySelector(element) : element;
        if (el) el.addEventListener(event, handler, options);
    },

    /**
     * 移除事件监听器
     * @param {Element|string} element 元素或选择器
     * @param {string} event 事件类型
     * @param {Function} handler 事件处理器
     */
    off: (element, event, handler) => {
        const el = typeof element === 'string' ? document.querySelector(element) : element;
        if (el) el.removeEventListener(event, handler);
    },

    /**
     * 触发自定义事件
     * @param {Element} element 元素
     * @param {string} event 事件类型
     * @param {*} detail 事件详情
     */
    trigger: (element, event, detail = null) => {
        const customEvent = new CustomEvent(event, {detail});
        element.dispatchEvent(customEvent);
    }
};

/**
 * 存储工具
 */
const storage = {
    /**
     * 本地存储设置
     * @param {string} key 键
     * @param {*} value 值
     */
    set: (key, value) => {
        try {
            localStorage.setItem(key, JSON.stringify(value));
        } catch (e) {
            console.warn('LocalStorage set failed:', e);
        }
    },

    /**
     * 本地存储获取
     * @param {string} key 键
     * @param {*} defaultValue 默认值
     * @returns {*} 存储的值
     */
    get: (key, defaultValue = null) => {
        try {
            const item = localStorage.getItem(key);
            return item ? JSON.parse(item) : defaultValue;
        } catch (e) {
            console.warn('LocalStorage get failed:', e);
            return defaultValue;
        }
    },

    /**
     * 本地存储删除
     * @param {string} key 键
     */
    remove: (key) => {
        try {
            localStorage.removeItem(key);
        } catch (e) {
            console.warn('LocalStorage remove failed:', e);
        }
    },

    /**
     * 清空本地存储
     */
    clear: () => {
        try {
            localStorage.clear();
        } catch (e) {
            console.warn('LocalStorage clear failed:', e);
        }
    }
};

// 导出工具函数
window.Utils = {
    debounce,
    throttle,
    getElementViewportPosition,
    smoothScrollTo,
    animateNumber,
    formatNumber,
    createSVGElement,
    random,
    randomColor,
    delay,
    isInViewport,
    typeCheck,
    dom,
    events,
    storage
}; 