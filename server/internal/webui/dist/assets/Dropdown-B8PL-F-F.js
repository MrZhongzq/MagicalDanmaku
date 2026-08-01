import{B as Ie,V as Ke,e as _e,r as $e,g as ze,p as fe}from"./bindings-Mr02UBcO.js";import{bA as Oe,af as De,aJ as W,w as de,bB as Ae,bC as Fe,o as Te,bu as q,r as j,c as ae,e as O,h as l,_ as X,i as B,a as k,b as P,d as z,z as he,a5 as le,u as ve,f as J,bD as je,x as be,n as m,T as Be,$ as G,bE as me,p as L,a0 as Me,bF as Ee,bG as Le,bH as He,a$ as Ue,Y as ue,m as $,a1 as Ve,bI as We,X as re,t as I,A as T}from"./index-IlwFrczE.js";import{f as qe,u as Ge}from"./_plugin-vue_export-helper-PfoTbyfu.js";import{C as Xe,h as ce,c as Je}from"./Select-f_U8x0zx.js";function Ye(e={},n){const d=De({ctrl:!1,command:!1,win:!1,shift:!1,tab:!1}),{keydown:r,keyup:t}=e,o=a=>{switch(a.key){case"Control":d.ctrl=!0;break;case"Meta":d.command=!0,d.win=!0;break;case"Shift":d.shift=!0;break;case"Tab":d.tab=!0;break}r!==void 0&&Object.keys(r).forEach(w=>{if(w!==a.key)return;const v=r[w];if(typeof v=="function")v(a);else{const{stop:g=!1,prevent:x=!1}=v;g&&a.stopPropagation(),x&&a.preventDefault(),v.handler(a)}})},s=a=>{switch(a.key){case"Control":d.ctrl=!1;break;case"Meta":d.command=!1,d.win=!1;break;case"Shift":d.shift=!1;break;case"Tab":d.tab=!1;break}t!==void 0&&Object.keys(t).forEach(w=>{if(w!==a.key)return;const v=t[w];if(typeof v=="function")v(a);else{const{stop:g=!1,prevent:x=!1}=v;g&&a.stopPropagation(),x&&a.preventDefault(),v.handler(a)}})},c=()=>{(n===void 0||n.value)&&(W("keydown",document,o),W("keyup",document,s)),n!==void 0&&de(n,a=>{a?(W("keydown",document,o),W("keyup",document,s)):(q("keydown",document,o),q("keyup",document,s))})};return Ae()?(Fe(c),Te(()=>{(n===void 0||n.value)&&(q("keydown",document,o),q("keyup",document,s))})):c(),Oe(d)}function Qe(e,n,d){const r=j(e.value);let t=null;return de(e,o=>{t!==null&&window.clearTimeout(t),o===!0?d&&!d.value?r.value=!0:t=window.setTimeout(()=>{r.value=!0},n):r.value=!1}),r}function Ze(e){return n=>{n?e.value=n.$el:e.value=null}}const se=ae("n-dropdown-menu"),Y=ae("n-dropdown"),pe=ae("n-dropdown-option"),we=O({name:"DropdownDivider",props:{clsPrefix:{type:String,required:!0}},render(){return l("div",{class:`${this.clsPrefix}-dropdown-divider`})}}),eo=O({name:"DropdownGroupHeader",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(){const{showIconRef:e,hasSubmenuRef:n}=B(se),{renderLabelRef:d,labelFieldRef:r,nodePropsRef:t,renderOptionRef:o}=B(Y);return{labelField:r,showIcon:e,hasSubmenu:n,renderLabel:d,nodeProps:t,renderOption:o}},render(){var e;const{clsPrefix:n,hasSubmenu:d,showIcon:r,nodeProps:t,renderLabel:o,renderOption:s}=this,{rawNode:c}=this.tmNode,a=l("div",Object.assign({class:`${n}-dropdown-option`},t==null?void 0:t(c)),l("div",{class:`${n}-dropdown-option-body ${n}-dropdown-option-body--group`},l("div",{"data-dropdown-option":!0,class:[`${n}-dropdown-option-body__prefix`,r&&`${n}-dropdown-option-body__prefix--show-icon`]},X(c.icon)),l("div",{class:`${n}-dropdown-option-body__label`,"data-dropdown-option":!0},o?o(c):X((e=c.title)!==null&&e!==void 0?e:c[this.labelField])),l("div",{class:[`${n}-dropdown-option-body__suffix`,d&&`${n}-dropdown-option-body__suffix--has-submenu`],"data-dropdown-option":!0})));return s?s({node:a,option:c}):a}}),oo=k("icon",`
 height: 1em;
 width: 1em;
 line-height: 1em;
 text-align: center;
 display: inline-block;
 position: relative;
 fill: currentColor;
`,[P("color-transition",{transition:"color .3s var(--n-bezier)"}),P("depth",{color:"var(--n-color)"},[z("svg",{opacity:"var(--n-opacity)",transition:"opacity .3s var(--n-bezier)"})]),z("svg",{height:"1em",width:"1em"})]),no=Object.assign(Object.assign({},J.props),{depth:[String,Number],size:[Number,String],color:String,component:[Object,Function]}),to=O({_n_icon__:!0,name:"Icon",inheritAttrs:!1,props:no,setup(e){const{mergedClsPrefixRef:n,inlineThemeDisabled:d}=ve(e),r=J("Icon","-icon",oo,je,e,n),t=m(()=>{const{depth:s}=e,{common:{cubicBezierEaseInOut:c},self:a}=r.value;if(s!==void 0){const{color:w,[`opacity${s}Depth`]:v}=a;return{"--n-bezier":c,"--n-color":w,"--n-opacity":v}}return{"--n-bezier":c,"--n-color":"","--n-opacity":""}}),o=d?be("icon",m(()=>`${e.depth||"d"}`),t,e):void 0;return{mergedClsPrefix:n,mergedStyle:m(()=>{const{size:s,color:c}=e;return{fontSize:qe(s),color:c}}),cssVars:d?void 0:t,themeClass:o==null?void 0:o.themeClass,onRender:o==null?void 0:o.onRender}},render(){var e;const{$parent:n,depth:d,mergedClsPrefix:r,component:t,onRender:o,themeClass:s}=this;return!((e=n==null?void 0:n.$options)===null||e===void 0)&&e._n_icon__&&he("icon","don't wrap `n-icon` inside `n-icon`"),o==null||o(),l("i",le(this.$attrs,{role:"img",class:[`${r}-icon`,s,{[`${r}-icon--depth`]:d,[`${r}-icon--color-transition`]:d!==void 0}],style:[this.cssVars,this.mergedStyle]}),t?l(t):this.$slots)}});function ie(e,n){return e.type==="submenu"||e.type===void 0&&e[n]!==void 0}function ro(e){return e.type==="group"}function ye(e){return e.type==="divider"}function io(e){return e.type==="render"}const ge=O({name:"DropdownOption",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null},placement:{type:String,default:"right-start"},props:Object,scrollable:Boolean},setup(e){const n=B(Y),{hoverKeyRef:d,keyboardKeyRef:r,lastToggledSubmenuKeyRef:t,pendingKeyPathRef:o,activeKeyPathRef:s,animatedRef:c,mergedShowRef:a,renderLabelRef:w,renderIconRef:v,labelFieldRef:g,childrenFieldRef:x,renderOptionRef:N,nodePropsRef:R,menuPropsRef:D}=n,S=B(pe,null),K=B(se),_=B(me),U=m(()=>e.tmNode.rawNode),H=m(()=>{const{value:i}=x;return ie(e.tmNode.rawNode,i)}),Q=m(()=>{const{disabled:i}=e.tmNode;return i}),Z=m(()=>{if(!H.value)return!1;const{key:i,disabled:f}=e.tmNode;if(f)return!1;const{value:y}=d,{value:A}=r,{value:te}=t,{value:F}=o;return y!==null?F.includes(i):A!==null?F.includes(i)&&F[F.length-1]!==i:te!==null?F.includes(i):!1}),ee=m(()=>r.value===null&&!c.value),oe=Qe(Z,300,ee),ne=m(()=>!!(S!=null&&S.enteringSubmenuRef.value)),M=j(!1);L(pe,{enteringSubmenuRef:M});function E(){M.value=!0}function V(){M.value=!1}function C(){const{parentKey:i,tmNode:f}=e;f.disabled||a.value&&(t.value=i,r.value=null,d.value=f.key)}function u(){const{tmNode:i}=e;i.disabled||a.value&&d.value!==i.key&&C()}function p(i){if(e.tmNode.disabled||!a.value)return;const{relatedTarget:f}=i;f&&!ce({target:f},"dropdownOption")&&!ce({target:f},"scrollbarRail")&&(d.value=null)}function h(){const{value:i}=H,{tmNode:f}=e;a.value&&!i&&!f.disabled&&(n.doSelect(f.key,f.rawNode),n.doUpdateShow(!1))}return{labelField:g,renderLabel:w,renderIcon:v,siblingHasIcon:K.showIconRef,siblingHasSubmenu:K.hasSubmenuRef,menuProps:D,popoverBody:_,animated:c,mergedShowSubmenu:m(()=>oe.value&&!ne.value),rawNode:U,hasSubmenu:H,pending:G(()=>{const{value:i}=o,{key:f}=e.tmNode;return i.includes(f)}),childActive:G(()=>{const{value:i}=s,{key:f}=e.tmNode,y=i.findIndex(A=>f===A);return y===-1?!1:y<i.length-1}),active:G(()=>{const{value:i}=s,{key:f}=e.tmNode,y=i.findIndex(A=>f===A);return y===-1?!1:y===i.length-1}),mergedDisabled:Q,renderOption:N,nodeProps:R,handleClick:h,handleMouseMove:u,handleMouseEnter:C,handleMouseLeave:p,handleSubmenuBeforeEnter:E,handleSubmenuAfterEnter:V}},render(){var e,n;const{animated:d,rawNode:r,mergedShowSubmenu:t,clsPrefix:o,siblingHasIcon:s,siblingHasSubmenu:c,renderLabel:a,renderIcon:w,renderOption:v,nodeProps:g,props:x,scrollable:N}=this;let R=null;if(t){const _=(e=this.menuProps)===null||e===void 0?void 0:e.call(this,r,r.children);R=l(xe,Object.assign({},_,{clsPrefix:o,scrollable:this.scrollable,tmNodes:this.tmNode.children,parentKey:this.tmNode.key}))}const D={class:[`${o}-dropdown-option-body`,this.pending&&`${o}-dropdown-option-body--pending`,this.active&&`${o}-dropdown-option-body--active`,this.childActive&&`${o}-dropdown-option-body--child-active`,this.mergedDisabled&&`${o}-dropdown-option-body--disabled`],onMousemove:this.handleMouseMove,onMouseenter:this.handleMouseEnter,onMouseleave:this.handleMouseLeave,onClick:this.handleClick},S=g==null?void 0:g(r),K=l("div",Object.assign({class:[`${o}-dropdown-option`,S==null?void 0:S.class],"data-dropdown-option":!0},S),l("div",le(D,x),[l("div",{class:[`${o}-dropdown-option-body__prefix`,s&&`${o}-dropdown-option-body__prefix--show-icon`]},[w?w(r):X(r.icon)]),l("div",{"data-dropdown-option":!0,class:`${o}-dropdown-option-body__label`},a?a(r):X((n=r[this.labelField])!==null&&n!==void 0?n:r.title)),l("div",{"data-dropdown-option":!0,class:[`${o}-dropdown-option-body__suffix`,c&&`${o}-dropdown-option-body__suffix--has-submenu`]},this.hasSubmenu?l(to,null,{default:()=>l(Xe,null)}):null)]),this.hasSubmenu?l(Ie,null,{default:()=>[l(Ke,null,{default:()=>l("div",{class:`${o}-dropdown-offset-container`},l(_e,{show:this.mergedShowSubmenu,placement:this.placement,to:N&&this.popoverBody||void 0,teleportDisabled:!N},{default:()=>l("div",{class:`${o}-dropdown-menu-wrapper`},d?l(Be,{onBeforeEnter:this.handleSubmenuBeforeEnter,onAfterEnter:this.handleSubmenuAfterEnter,name:"fade-in-scale-up-transition",appear:!0},{default:()=>R}):R)}))})]}):null);return v?v({node:K,option:r}):K}}),ao=O({name:"NDropdownGroup",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null}},render(){const{tmNode:e,parentKey:n,clsPrefix:d}=this,{children:r}=e;return l(Me,null,l(eo,{clsPrefix:d,tmNode:e,key:e.key}),r==null?void 0:r.map(t=>{const{rawNode:o}=t;return o.show===!1?null:ye(o)?l(we,{clsPrefix:d,key:t.key}):t.isGroup?(he("dropdown","`group` node is not allowed to be put in `group` node."),null):l(ge,{clsPrefix:d,tmNode:t,parentKey:n,key:t.key})}))}}),lo=O({name:"DropdownRenderOption",props:{tmNode:{type:Object,required:!0}},render(){const{rawNode:{render:e,props:n}}=this.tmNode;return l("div",n,[e==null?void 0:e()])}}),xe=O({name:"DropdownMenu",props:{scrollable:Boolean,showArrow:Boolean,arrowStyle:[String,Object],clsPrefix:{type:String,required:!0},tmNodes:{type:Array,default:()=>[]},parentKey:{type:[String,Number],default:null}},setup(e){const{renderIconRef:n,childrenFieldRef:d}=B(Y);L(se,{showIconRef:m(()=>{const t=n.value;return e.tmNodes.some(o=>{var s;if(o.isGroup)return(s=o.children)===null||s===void 0?void 0:s.some(({rawNode:a})=>t?t(a):a.icon);const{rawNode:c}=o;return t?t(c):c.icon})}),hasSubmenuRef:m(()=>{const{value:t}=d;return e.tmNodes.some(o=>{var s;if(o.isGroup)return(s=o.children)===null||s===void 0?void 0:s.some(({rawNode:a})=>ie(a,t));const{rawNode:c}=o;return ie(c,t)})})});const r=j(null);return L(Le,null),L(He,null),L(me,r),{bodyRef:r}},render(){const{parentKey:e,clsPrefix:n,scrollable:d}=this,r=this.tmNodes.map(t=>{const{rawNode:o}=t;return o.show===!1?null:io(o)?l(lo,{tmNode:t,key:t.key}):ye(o)?l(we,{clsPrefix:n,key:t.key}):ro(o)?l(ao,{clsPrefix:n,tmNode:t,parentKey:e,key:t.key}):l(ge,{clsPrefix:n,tmNode:t,parentKey:e,key:t.key,props:o.props,scrollable:d})});return l("div",{class:[`${n}-dropdown-menu`,d&&`${n}-dropdown-menu--scrollable`],ref:"bodyRef"},d?l(Ee,{contentClass:`${n}-dropdown-menu__content`},{default:()=>r}):r,this.showArrow?$e({clsPrefix:n,arrowStyle:this.arrowStyle,arrowClass:void 0,arrowWrapperClass:void 0,arrowWrapperStyle:void 0}):null)}}),so=k("dropdown-menu",`
 transform-origin: var(--v-transform-origin);
 background-color: var(--n-color);
 border-radius: var(--n-border-radius);
 box-shadow: var(--n-box-shadow);
 position: relative;
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
`,[Ue(),k("dropdown-option",`
 position: relative;
 `,[z("a",`
 text-decoration: none;
 color: inherit;
 outline: none;
 `,[z("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),k("dropdown-option-body",`
 display: flex;
 cursor: pointer;
 position: relative;
 height: var(--n-option-height);
 line-height: var(--n-option-height);
 font-size: var(--n-font-size);
 color: var(--n-option-text-color);
 transition: color .3s var(--n-bezier);
 `,[z("&::before",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 left: 4px;
 right: 4px;
 transition: background-color .3s var(--n-bezier);
 border-radius: var(--n-border-radius);
 `),ue("disabled",[P("pending",`
 color: var(--n-option-text-color-hover);
 `,[$("prefix, suffix",`
 color: var(--n-option-text-color-hover);
 `),z("&::before","background-color: var(--n-option-color-hover);")]),P("active",`
 color: var(--n-option-text-color-active);
 `,[$("prefix, suffix",`
 color: var(--n-option-text-color-active);
 `),z("&::before","background-color: var(--n-option-color-active);")]),P("child-active",`
 color: var(--n-option-text-color-child-active);
 `,[$("prefix, suffix",`
 color: var(--n-option-text-color-child-active);
 `)])]),P("disabled",`
 cursor: not-allowed;
 opacity: var(--n-option-opacity-disabled);
 `),P("group",`
 font-size: calc(var(--n-font-size) - 1px);
 color: var(--n-group-header-text-color);
 `,[$("prefix",`
 width: calc(var(--n-option-prefix-width) / 2);
 `,[P("show-icon",`
 width: calc(var(--n-option-icon-prefix-width) / 2);
 `)])]),$("prefix",`
 width: var(--n-option-prefix-width);
 display: flex;
 justify-content: center;
 align-items: center;
 color: var(--n-prefix-color);
 transition: color .3s var(--n-bezier);
 z-index: 1;
 `,[P("show-icon",`
 width: var(--n-option-icon-prefix-width);
 `),k("icon",`
 font-size: var(--n-option-icon-size);
 `)]),$("label",`
 white-space: nowrap;
 flex: 1;
 z-index: 1;
 `),$("suffix",`
 box-sizing: border-box;
 flex-grow: 0;
 flex-shrink: 0;
 display: flex;
 justify-content: flex-end;
 align-items: center;
 min-width: var(--n-option-suffix-width);
 padding: 0 8px;
 transition: color .3s var(--n-bezier);
 color: var(--n-suffix-color);
 z-index: 1;
 `,[P("has-submenu",`
 width: var(--n-option-icon-suffix-width);
 `),k("icon",`
 font-size: var(--n-option-icon-size);
 `)]),k("dropdown-menu","pointer-events: all;")]),k("dropdown-offset-container",`
 pointer-events: none;
 position: absolute;
 left: 0;
 right: 0;
 top: -4px;
 bottom: -4px;
 `)]),k("dropdown-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 4px 0;
 `),k("dropdown-menu-wrapper",`
 transform-origin: var(--v-transform-origin);
 width: fit-content;
 `),z(">",[k("scrollbar",`
 height: inherit;
 max-height: inherit;
 `)]),ue("scrollable",`
 padding: var(--n-padding);
 `),P("scrollable",[$("content",`
 padding: var(--n-padding);
 `)])]),uo={animated:{type:Boolean,default:!0},keyboard:{type:Boolean,default:!0},size:String,inverted:Boolean,placement:{type:String,default:"bottom"},onSelect:[Function,Array],options:{type:Array,default:()=>[]},menuProps:Function,showArrow:Boolean,renderLabel:Function,renderIcon:Function,renderOption:Function,nodeProps:Function,labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},value:[String,Number]},co=Object.keys(fe),po=Object.assign(Object.assign(Object.assign({},fe),uo),J.props),mo=O({name:"Dropdown",inheritAttrs:!1,props:po,setup(e){const n=j(!1),d=Ge(I(e,"show"),n),r=m(()=>{const{keyField:u,childrenField:p}=e;return Je(e.options,{getKey(h){return h[u]},getDisabled(h){return h.disabled===!0},getIgnored(h){return h.type==="divider"||h.type==="render"},getChildren(h){return h[p]}})}),t=m(()=>r.value.treeNodes),o=j(null),s=j(null),c=j(null),a=m(()=>{var u,p,h;return(h=(p=(u=o.value)!==null&&u!==void 0?u:s.value)!==null&&p!==void 0?p:c.value)!==null&&h!==void 0?h:null}),w=m(()=>r.value.getPath(a.value).keyPath),v=m(()=>r.value.getPath(e.value).keyPath),g=G(()=>e.keyboard&&d.value);Ye({keydown:{ArrowUp:{prevent:!0,handler:ee},ArrowRight:{prevent:!0,handler:Z},ArrowDown:{prevent:!0,handler:oe},ArrowLeft:{prevent:!0,handler:Q},Enter:{prevent:!0,handler:ne},Escape:H}},g);const{mergedClsPrefixRef:x,inlineThemeDisabled:N,mergedComponentPropsRef:R}=ve(e),D=m(()=>{var u,p;return e.size||((p=(u=R==null?void 0:R.value)===null||u===void 0?void 0:u.Dropdown)===null||p===void 0?void 0:p.size)||"medium"}),S=J("Dropdown","-dropdown",so,We,e,x);L(Y,{labelFieldRef:I(e,"labelField"),childrenFieldRef:I(e,"childrenField"),renderLabelRef:I(e,"renderLabel"),renderIconRef:I(e,"renderIcon"),hoverKeyRef:o,keyboardKeyRef:s,lastToggledSubmenuKeyRef:c,pendingKeyPathRef:w,activeKeyPathRef:v,animatedRef:I(e,"animated"),mergedShowRef:d,nodePropsRef:I(e,"nodeProps"),renderOptionRef:I(e,"renderOption"),menuPropsRef:I(e,"menuProps"),doSelect:K,doUpdateShow:_}),de(d,u=>{!e.animated&&!u&&U()});function K(u,p){const{onSelect:h}=e;h&&re(h,u,p)}function _(u){const{"onUpdate:show":p,onUpdateShow:h}=e;p&&re(p,u),h&&re(h,u),n.value=u}function U(){o.value=null,s.value=null,c.value=null}function H(){_(!1)}function Q(){E("left")}function Z(){E("right")}function ee(){E("up")}function oe(){E("down")}function ne(){const u=M();u!=null&&u.isLeaf&&d.value&&(K(u.key,u.rawNode),_(!1))}function M(){var u;const{value:p}=r,{value:h}=a;return!p||h===null?null:(u=p.getNode(h))!==null&&u!==void 0?u:null}function E(u){const{value:p}=a,{value:{getFirstAvailableNode:h}}=r;let i=null;if(p===null){const f=h();f!==null&&(i=f.key)}else{const f=M();if(f){let y;switch(u){case"down":y=f.getNext();break;case"up":y=f.getPrev();break;case"right":y=f.getChild();break;case"left":y=f.getParent();break}y&&(i=y.key)}}i!==null&&(o.value=null,s.value=i)}const V=m(()=>{const{inverted:u}=e,p=D.value,{common:{cubicBezierEaseInOut:h},self:i}=S.value,{padding:f,dividerColor:y,borderRadius:A,optionOpacityDisabled:te,[T("optionIconSuffixWidth",p)]:F,[T("optionSuffixWidth",p)]:Se,[T("optionIconPrefixWidth",p)]:ke,[T("optionPrefixWidth",p)]:Pe,[T("fontSize",p)]:Ne,[T("optionHeight",p)]:Re,[T("optionIconSize",p)]:Ce}=i,b={"--n-bezier":h,"--n-font-size":Ne,"--n-padding":f,"--n-border-radius":A,"--n-option-height":Re,"--n-option-prefix-width":Pe,"--n-option-icon-prefix-width":ke,"--n-option-suffix-width":Se,"--n-option-icon-suffix-width":F,"--n-option-icon-size":Ce,"--n-divider-color":y,"--n-option-opacity-disabled":te};return u?(b["--n-color"]=i.colorInverted,b["--n-option-color-hover"]=i.optionColorHoverInverted,b["--n-option-color-active"]=i.optionColorActiveInverted,b["--n-option-text-color"]=i.optionTextColorInverted,b["--n-option-text-color-hover"]=i.optionTextColorHoverInverted,b["--n-option-text-color-active"]=i.optionTextColorActiveInverted,b["--n-option-text-color-child-active"]=i.optionTextColorChildActiveInverted,b["--n-prefix-color"]=i.prefixColorInverted,b["--n-suffix-color"]=i.suffixColorInverted,b["--n-group-header-text-color"]=i.groupHeaderTextColorInverted):(b["--n-color"]=i.color,b["--n-option-color-hover"]=i.optionColorHover,b["--n-option-color-active"]=i.optionColorActive,b["--n-option-text-color"]=i.optionTextColor,b["--n-option-text-color-hover"]=i.optionTextColorHover,b["--n-option-text-color-active"]=i.optionTextColorActive,b["--n-option-text-color-child-active"]=i.optionTextColorChildActive,b["--n-prefix-color"]=i.prefixColor,b["--n-suffix-color"]=i.suffixColor,b["--n-group-header-text-color"]=i.groupHeaderTextColor),b}),C=N?be("dropdown",m(()=>`${D.value[0]}${e.inverted?"i":""}`),V,e):void 0;return{mergedClsPrefix:x,mergedTheme:S,mergedSize:D,tmNodes:t,mergedShow:d,handleAfterLeave:()=>{e.animated&&U()},doUpdateShow:_,cssVars:N?void 0:V,themeClass:C==null?void 0:C.themeClass,onRender:C==null?void 0:C.onRender}},render(){const e=(r,t,o,s,c)=>{var a;const{mergedClsPrefix:w,menuProps:v}=this;(a=this.onRender)===null||a===void 0||a.call(this);const g=(v==null?void 0:v(void 0,this.tmNodes.map(N=>N.rawNode)))||{},x={ref:Ze(t),class:[r,`${w}-dropdown`,`${w}-dropdown--${this.mergedSize}-size`,this.themeClass],clsPrefix:w,tmNodes:this.tmNodes,style:[...o,this.cssVars],showArrow:this.showArrow,arrowStyle:this.arrowStyle,scrollable:this.scrollable,onMouseenter:s,onMouseleave:c};return l(xe,le(this.$attrs,x,g))},{mergedTheme:n}=this,d={show:this.mergedShow,theme:n.peers.Popover,themeOverrides:n.peerOverrides.Popover,internalOnAfterLeave:this.handleAfterLeave,internalRenderBody:e,onUpdateShow:this.doUpdateShow,"onUpdate:show":void 0};return l(ze,Object.assign({},Ve(this.$props,co),d),{trigger:()=>{var r,t;return(t=(r=this.$slots).default)===null||t===void 0?void 0:t.call(r)}})}});export{mo as N,Ze as c,Ye as u};
